package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/view"
	log "github.com/sirupsen/logrus"
)

type ProblemsService interface {
	GenTaskRestDocProblems(ctx context.Context, packageId string, version string, revision int, slug string, docData string) ([]view.AIApiDocCatProblem, error)
	GetDocProblems(ctx context.Context, packageId string, version string, slug string) ([]view.AIApiDocCatProblem, error)
	GenTaskRestOpProblems(ctx context.Context, packageId string, version string, revision int, slug string, operationId string, opData string) ([]view.AIApiDocCatProblem, error)
	GetOpProblems(ctx context.Context, packageId string, version string, slug string, operationId string) ([]view.AIApiDocCatProblem, error)

	GetVersionIssues(ctx context.Context, packageId string, version string) (view.VersionIssues, error)
}

func NewProblemsService(apihubClient client.ApihubClient, llmClient client.LLMClient, operationResultRepository repository.OperationResultRepository, versionResultRepository repository.VersionResultRepository, problemsRepository repository.ProblemsRepository, localFileStore bool) ProblemsService {

	storage := make(map[string][]view.AIApiDocCatProblem)

	if localFileStore {
		data, err := os.ReadFile("problems_storage.json")
		if err == nil {
			if err := json.Unmarshal(data, &storage); err != nil {
				log.Errorf("Warning: Failed to unmarshal storage file: %v", err)
			}
		} else {
			log.Warnf("Warning: Failed to read storage file: %v", err)
		}
	}

	return &problemsServiceImpl{
		apihubClient:              apihubClient,
		llmClient:                 llmClient,
		operationResultRepository: operationResultRepository,
		versionResultRepository:   versionResultRepository,
		problemsRepository:        problemsRepository,
		localFileStore:            localFileStore,
		storage:                   storage,
	}
}

type problemsServiceImpl struct {
	apihubClient              client.ApihubClient
	llmClient                 client.LLMClient
	operationResultRepository repository.OperationResultRepository
	versionResultRepository   repository.VersionResultRepository
	problemsRepository        repository.ProblemsRepository

	localFileStore bool
	storage        map[string][]view.AIApiDocCatProblem
}

func (p problemsServiceImpl) GetVersionIssues(ctx context.Context, packageId string, version string) (view.VersionIssues, error) {
	result := view.VersionIssues{
		LinterIssues: []view.OperationResult{},
		AIProblems:   []view.AIApiDocCatProblem{},
	}

	ver, rev, err := getVersionAndRevision(ctx, p.apihubClient, packageId, version)
	if err != nil {
		return result, err
	}

	// Get all operations for the version
	operations, err := p.operationResultRepository.GetOperationsForVersion(ctx, packageId, ver)
	if err != nil {
		return result, err
	}

	// Convert operations to OperationResult view and collect linter issues
	versionStr := fmt.Sprintf("%s@%d", ver, rev)
	for _, op := range operations {
		// Filter by revision to get only operations for the specific revision
		if op.Revision != rev {
			continue
		}

		// Skip operations without data hash (no lint result)
		if op.DataHash == "" {
			continue
		}

		// Get operation result
		opResult, err := p.operationResultRepository.GetOperationResult(ctx, op.DataHash, op.RulesetId)
		if err != nil {
			log.Warnf("Failed to get operation result for operation %s: %v", op.OperationId, err)
			continue
		}
		if opResult == nil {
			continue
		}

		// Get operation details from API hub
		// Convert ApiType to OpApiType - for OpenAPI documents, operations are REST
		opApiType := view.RestApiType

		var validatedOp view.ValidatedOperation
		operation, err := p.apihubClient.GetOperationWithData(ctx, packageId, versionStr, opApiType, op.OperationId)
		if err != nil {
			log.Warnf("Failed to get operation details for operation %s: %v", op.OperationId, err)
			// Still create the result with available data
			validatedOp = view.ValidatedOperation{
				DocSlug:     op.Slug,
				OperationId: op.OperationId,
				ApiType:     op.SpecificationType,
				DocName:     op.FileId,
			}
		} else {
			validatedOp = view.ValidatedOperation{
				DocSlug:     op.Slug,
				OperationId: op.OperationId,
				Title:       operation.Title,
				Path:        operation.Path,
				Method:      operation.Method,
				ApiType:     op.SpecificationType,
				DocName:     op.FileId,
			}
		}

		// Convert spectral output to validation issues
		issues := make([]view.ValidationIssue, 0)
		var spectralOutput []view.SpectralOutputItem
		err = json.Unmarshal(opResult.Data, &spectralOutput)
		if err != nil {
			log.Warnf("Failed to unmarshal operation result data for operation %s: %v", op.OperationId, err)
			continue
		}

		for _, item := range spectralOutput {
			var path []string
			if item.Path != nil {
				path = item.Path
			} else {
				path = make([]string, 0)
			}
			issues = append(issues, view.ValidationIssue{
				Path:     path,
				Code:     item.Code,
				Severity: view.ConvertSpectralSeverityToString(item.Severity),
				Message:  item.Message,
			})
		}

		result.LinterIssues = append(result.LinterIssues, view.OperationResult{
			Issues:             issues,
			ValidatedOperation: validatedOp,
		})
	}

	// Collect AI problems for all operations (including those without linter results)
	for _, op := range operations {
		// Filter by revision to get only operations for the specific revision
		if op.Revision != rev {
			continue
		}

		// Try to get from database first
		ent, err := p.problemsRepository.GetProblems(ctx, packageId, ver, rev, op.OperationId)
		if err != nil {
			log.Warnf("Failed to get problems from database for operation %s: %v", op.OperationId, err)
		} else if len(ent.Problems) > 0 {
			result.AIProblems = append(result.AIProblems, ent.Problems...)
			continue
		}

		// Fallback to local file store if enabled
		if p.localFileStore {
			opKey := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + op.Slug + "|" + op.OperationId
			if aiProblems, exists := p.storage[opKey]; exists {
				result.AIProblems = append(result.AIProblems, aiProblems...)
			}
		}
	}

	// Get all documents for the version to collect AI problems for documents
	_, docs, err := p.versionResultRepository.GetVersionAndDocsSummary(ctx, packageId, ver, rev)
	if err != nil {
		log.Warnf("Failed to get documents for version: %v", err)
		// Continue without document AI problems if we can't get documents
	} else {
		// Collect AI problems for all documents
		for _, doc := range docs {
			key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + doc.Slug
			if aiProblems, exists := p.storage[key]; exists {
				result.AIProblems = append(result.AIProblems, aiProblems...)
			}
		}
	}

	return result, nil
}

func (p problemsServiceImpl) GenTaskRestDocProblems(ctx context.Context, packageId string, version string, revision int, slug string, docData string) ([]view.AIApiDocCatProblem, error) {
	start := time.Now()
	log.Infof("Run detect problems with openai client for doc %s %s@%d %s", packageId, version, revision, slug)
	problResp, _, err := p.llmClient.GenerateProblems(ctx, docData)
	log.Infof("finished detect problems with openai client for doc %s %s@%d %s, it took %dms", packageId, version, revision, slug, time.Since(start).Milliseconds())
	if err != nil {
		return nil, err
	}

	catProbl, err := p.llmClient.CategorizeProblems(ctx, problResp)
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(catProbl, compareProblemsBySeverity)

	key := packageId + "|" + fmt.Sprintf("%s@%d", version, revision) + "|" + slug
	p.storage[key] = catProbl

	log.Infof("time: %dms", time.Since(start).Milliseconds())
	log.Infof("problems: %+v", problResp)
	p.saveStorage()

	return catProbl, nil
}

func (p problemsServiceImpl) GetDocProblems(ctx context.Context, packageId string, version string, slug string) ([]view.AIApiDocCatProblem, error) {
	ver, rev, err := getVersionAndRevision(ctx, p.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	// Get all problems for operations in the document
	problemsList, err := p.problemsRepository.GetProblemsForDoc(ctx, packageId, ver, rev, slug)
	if err != nil {
		log.Warnf("Failed to get problems from database for document %s: %v", slug, err)
		return []view.AIApiDocCatProblem{}, nil
	}

	// Sum up problems from all operations, excluding duplicates
	seen := make(map[string]bool)
	result := make([]view.AIApiDocCatProblem, 0)
	for _, problemsEntity := range problemsList {
		for _, problem := range problemsEntity.Problems {
			// Create a unique key from Text, Severity, and Category
			key := fmt.Sprintf("%s|%s|%s", problem.Text, problem.Severity, problem.Category)
			if !seen[key] {
				seen[key] = true
				result = append(result, problem)
			}
		}
	}

	slices.SortStableFunc(result, compareProblemsBySeverity)

	return result, nil
}

var severityOrder = map[string]int{
	view.PSError:   0,
	view.PSWarning: 1,
	view.PSInfo:    2,
}

func compareProblemsBySeverity(a, b view.AIApiDocCatProblem) int {
	aw, ok := severityOrder[a.Severity]
	if !ok {
		aw = len(severityOrder)
	}
	bw, ok := severityOrder[b.Severity]
	if !ok {
		bw = len(severityOrder)
	}

	switch {
	case aw < bw:
		return -1
	case aw > bw:
		return 1
	default:
		return 0
	}
}

func (p problemsServiceImpl) GenTaskRestOpProblems(ctx context.Context, packageId string, version string, revision int, slug string, operationId string, opData string) ([]view.AIApiDocCatProblem, error) {
	start := time.Now()
	log.Infof("Run detect problems with openai client for operation %s %s@%d %s", packageId, version, revision, operationId)
	problResp, promptHash, err := p.llmClient.GenerateProblems(ctx, opData)
	log.Infof("finished detect problems with openai client for operation %s %s@%d %s, it took %dms", packageId, version, revision, operationId, time.Since(start).Milliseconds())
	if err != nil {
		return nil, err
	}

	catProbl, err := p.llmClient.CategorizeProblems(ctx, problResp)
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(catProbl, func(a, b view.AIApiDocCatProblem) int {
		switch a.Severity {
		case view.PSError:
			switch b.Severity {
			case view.PSError:
				return 0
			case view.PSWarning:
				return -1
			case view.PSInfo:
				return -1
			}
		case view.PSWarning:
			switch b.Severity {
			case view.PSError:
				return 1
			case view.PSWarning:
				return 0
			case view.PSInfo:
				return -1
			}
		case view.PSInfo:
			switch b.Severity {
			case view.PSError:
				return 1
			case view.PSWarning:
				return 1
			case view.PSInfo:
				return 0
			}
		}
		return 0
	})

	// Save to database
	ent := entity.Problems{
		PackageId:   packageId,
		Version:     version,
		Revision:    revision,
		OperationId: operationId,
		FileSlug:    slug,
		PromptHash:  promptHash,
		Problems:    catProbl,
	}
	err = p.problemsRepository.SaveProblems(ctx, ent)
	if err != nil {
		return nil, fmt.Errorf("failed to save problems to database: %v", err)
	}

	log.Infof("time: %dms", time.Since(start).Milliseconds())
	log.Infof("problems: %+v", problResp)

	return catProbl, nil
}

func (p problemsServiceImpl) GetOpProblems(ctx context.Context, packageId string, version string, slug string, operationId string) ([]view.AIApiDocCatProblem, error) {
	ver, rev, err := getVersionAndRevision(ctx, p.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	// Try to get from database first
	ent, err := p.problemsRepository.GetProblems(ctx, packageId, ver, rev, operationId)
	if err != nil {
		log.Warnf("Failed to get problems from database: %v", err)
	} else if len(ent.Problems) > 0 {
		return ent.Problems, nil
	}

	// Fallback to local file store if enabled
	if p.localFileStore {
		key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug + "|" + operationId
		result, exists := p.storage[key]
		if exists {
			return result, nil
		}
	}

	return []view.AIApiDocCatProblem{}, nil
}

func (p problemsServiceImpl) saveStorage() {
	if !p.localFileStore {
		return
	}

	data, err := json.Marshal(p.storage)
	if err != nil {
		log.Errorf("err: %+v", err)
		return
	}

	err = os.WriteFile("problems_storage.json", data, 0644)
	if err != nil {
		log.Errorf("Failed to save storage to file: %+v", err)
	}
}
