package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/utils"
	"github.com/Netcracker/qubership-api-linter-service/view"
	log "github.com/sirupsen/logrus"
)

type EnhancementService interface {
	EnhanceDoc(ctx context.Context, packageId string, version string, slug string) error

	GetEnhanceStatus(ctx context.Context, packageId string, version string, slug string) (*view.EnhancementStatusResponse, error)
	GetEnhancedDoc(ctx context.Context, packageId string, version string, slug string) (string, error)

	UpdateEnhancedDoc(ctx context.Context, packageId string, version string, slug string, data string) error

	PublishEnhancedDocs(ctx context.Context, packageId string, version string, req view.PublishEnhancementRequest) (*view.PublishResponse, error)
}

func NewEnhancementService(apihubClient client.ApihubClient, llmClient client.LLMClient, problemsService ProblemsService,
	validationService ValidationService, scoringService ScoringService, spectralExecutor SpectralExecutor, ruleSetRepository repository.RulesetRepository, localFileStore bool) EnhancementService {

	status := make(map[string]view.EnhancementStatusResponse)
	enhancedDocs := make(map[string]string)

	if localFileStore {
		data, err := os.ReadFile("enhance_status.json")
		if err == nil {
			if err := json.Unmarshal(data, &status); err != nil {
				log.Errorf("Warning: Failed to unmarshal storage file: %v", err)
			}
		} else {
			log.Warnf("Warning: Failed to read storage file: %v", err)
		}

		data, err = os.ReadFile("enhance_docs.json")
		if err == nil {
			if err := json.Unmarshal(data, &enhancedDocs); err != nil {
				log.Errorf("Warning: Failed to unmarshal storage file: %v", err)
			}
		} else {
			log.Warnf("Warning: Failed to read storage file: %v", err)
		}
	}

	return &enhancementServiceImpl{
		apihubClient:      apihubClient,
		llmClient:         llmClient,
		problemsService:   problemsService,
		validationService: validationService,
		scoringService:    scoringService,
		spectralExecutor:  spectralExecutor,
		ruleSetRepository: ruleSetRepository,

		localFileStore: localFileStore,
		status:         status,
		enhancedDocs:   enhancedDocs,
	}
}

type enhancementServiceImpl struct {
	apihubClient      client.ApihubClient
	llmClient         client.LLMClient
	problemsService   ProblemsService
	validationService ValidationService
	scoringService    ScoringService
	spectralExecutor  SpectralExecutor
	ruleSetRepository repository.RulesetRepository

	// TODO: temp!
	localFileStore bool
	status         map[string]view.EnhancementStatusResponse
	enhancedDocs   map[string]string
}

func (e enhancementServiceImpl) EnhanceDoc(ctx context.Context, packageId string, version string, slug string) error {

	ver, rev, err := getVersionAndRevision(ctx, e.apihubClient, packageId, version)
	if err != nil {
		return err
	}
	version = fmt.Sprintf("%s@%d", ver, rev)

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug

	// check status first!
	status, ok := e.status[key]
	if ok {
		if status.Status == view.ESSuccess || status.Status == view.ESProcessing {
			return nil
		}
	}

	data, err := e.apihubClient.GetDocumentRawData(ctx, packageId, version, slug)
	if err != nil {
		// TODO: save status to storage ???
		return err
	}

	if len(data) == 0 {
		// TODO: save status to storage ???
		return fmt.Errorf("doc data is empty")
	}

	problems, err := e.problemsService.GetDocProblems(ctx, packageId, version, slug)
	if err != nil {
		// TODO: save status to storage
		return fmt.Errorf("Failed to get doc problems")
	}

	validationResult, err := e.validationService.GetValidationResult(ctx, packageId, version, slug)
	if err != nil {
		// TODO: save status to storage
		return fmt.Errorf("Failed to get validation result")
	}

	e.status[key] = view.EnhancementStatusResponse{
		Status:  view.ESProcessing,
		Details: "",
	}

	utils.SafeAsync(func() {
		start := time.Now()

		asyncContext := context.Background()

		fixedDoc, err := e.llmClient.FixProblems(asyncContext, string(data), problems, validationResult.Issues)
		if err != nil {
			log.Errorf("Failed to generate fixed doc: %s", err)
			e.status[key] = view.EnhancementStatusResponse{
				Status:  view.ESError,
				Details: fmt.Sprintf("Failed to generate fixed doc: %s", err),
			}
			e.saveStorage()
			return
		}
		log.Infof("enhance time: %dms", time.Since(start).Milliseconds())
		log.Infof("fixedDoc: %+v", fixedDoc)

		/*err = saveDebugData(task, docData, lintSummary, lintReport, problResp, fixedDoc)
		if err != nil {
			return nil, err
		}*/

		docSummary, err := e.lintEnhancedDoc(asyncContext, packageId, version, slug, validationResult.Ruleset.Id, fixedDoc)
		if err != nil {
			log.Errorf("Failed to lint enhanced doc: %s", err)
			e.status[key] = view.EnhancementStatusResponse{
				Status:  view.ESError,
				Details: fmt.Sprintf("Failed to lint enhanced doc: %s", err),
			}
			e.saveStorage()
			return
		}

		_, err = e.scoringService.MakeEnhancedRestDocScore(asyncContext, packageId, version, slug, fixedDoc, *docSummary)
		if err != nil {
			log.Errorf("Failed to generate enhanced doc score")
			e.status[key] = view.EnhancementStatusResponse{
				Status:  view.ESError,
				Details: fmt.Sprintf("Failed to generate enhanced doc score"),
			}
			e.saveStorage()
			return
		}

		e.status[key] = view.EnhancementStatusResponse{
			Status:  view.ESSuccess,
			Details: "",
		}
		e.enhancedDocs[key] = fixedDoc
		e.saveStorage()
	})

	return nil
}

func (e enhancementServiceImpl) GetEnhanceStatus(ctx context.Context, packageId string, version string, slug string) (*view.EnhancementStatusResponse, error) {
	ver, rev, err := getVersionAndRevision(ctx, e.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug

	status, ok := e.status[key]
	if !ok {
		status = view.EnhancementStatusResponse{
			Status:  view.ESNotStarted,
			Details: "",
		}
	}
	return &status, nil
}

func (e enhancementServiceImpl) GetEnhancedDoc(ctx context.Context, packageId string, version string, slug string) (string, error) {
	ver, rev, err := getVersionAndRevision(ctx, e.apihubClient, packageId, version)
	if err != nil {
		return "", err
	}

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug

	return e.enhancedDocs[key], nil
}

func (e enhancementServiceImpl) UpdateEnhancedDoc(ctx context.Context, packageId string, version string, slug string, data string) error {
	ver, rev, err := getVersionAndRevision(ctx, e.apihubClient, packageId, version)
	if err != nil {
		return err
	}

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug

	e.enhancedDocs[key] = data
	return nil
}

func (e enhancementServiceImpl) PublishEnhancedDocs(ctx context.Context, packageId string, version string, req view.PublishEnhancementRequest) (*view.PublishResponse, error) {
	// download version sources (archive and config)
	src, err := e.apihubClient.GetVersionSources(ctx, packageId, version)
	if err != nil {
		return nil, err
	}

	// iterate over version docs and find all enhanced docs
	ver, rev, err := getVersionAndRevision(ctx, e.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}
	version = fmt.Sprintf("%s@%d", ver, rev)

	enhancedDocKeys := make(map[string]string) // fileId to key

	docs, err := e.apihubClient.GetVersionDocuments(ctx, packageId, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get version documents: %s", err)
	}
	if docs == nil {
		return nil, fmt.Errorf("failed to get version documents: not found")
	}

	for _, doc := range docs.Documents {
		key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + doc.Slug
		if _, ok := e.enhancedDocs[key]; ok {
			enhancedDocKeys[doc.FileId] = key
		}
	}

	// make new build with replaced files

	// Create a zip reader from the decoded data
	zipReader, err := zip.NewReader(bytes.NewReader(src.Sources), int64(len(src.Sources)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %w", err)
	}

	// Create a buffer for the new zip file
	var newZipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&newZipBuffer)

	// replace file content with enhanced once based on fileId(path)
	for _, file := range zipReader.File {
		// Open the file from the original zip
		originalFile, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s from zip: %w", file.Name, err)
		}

		var fileContent []byte
		// Check if this file needs to be enhanced

		key, shouldEnhance := enhancedDocKeys[file.Name]

		if shouldEnhance {
			fileContent = []byte(e.enhancedDocs[key])
		} else {
			// Read original content
			fileContent, err = io.ReadAll(originalFile)
			if err != nil {
				originalFile.Close()
				return nil, fmt.Errorf("failed to read file %s: %w", file.Name, err)
			}
		}

		originalFile.Close()

		// Create the file in the new zip
		zipFile, err := zipWriter.Create(file.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s in new zip: %w", file.Name, err)
		}

		// Write content to the new zip file
		_, err = zipFile.Write(fileContent)
		if err != nil {
			return nil, fmt.Errorf("failed to write file %s to new zip: %w", file.Name, err)
		}
	}

	// Close the zip writer
	err = zipWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	data := newZipBuffer.Bytes()

	src.Config.PackageId = req.PackageId
	src.Config.Version = req.Version
	src.Config.PreviousVersion = req.PreviousVersion
	src.Config.Status = string(req.Status)
	src.Config.Metadata.VersionLabels = req.Labels
	src.Config.MigrationBuild = false
	src.Config.MigrationId = ""
	src.Config.NoChangelog = false
	src.Config.PublishedAt = time.Time{}
	src.Config.PublishId = ""

	buildId, err := e.apihubClient.PublishVersion(ctx, src.Config, data)
	if err != nil {
		return nil, err
	}
	resp := view.PublishResponse{
		PublishId: buildId,
	}

	return &resp, nil
}

func (e enhancementServiceImpl) saveStorage() {
	if !e.localFileStore {
		return
	}
	data, err := json.Marshal(e.status)
	if err != nil {
		log.Errorf("err: %+v", err)
		return
	}

	err = os.WriteFile("enhance_status.json", data, 0644)
	if err != nil {
		log.Errorf("Failed to save status storage to file: %+v", err)
	}

	data, err = json.Marshal(e.enhancedDocs)
	if err != nil {
		log.Errorf("err: %+v", err)
		return
	}

	err = os.WriteFile("enhance_docs.json", data, 0644)
	if err != nil {
		log.Errorf("Failed to save docs storage to file: %+v", err)
	}
}

func (e enhancementServiceImpl) lintEnhancedDoc(ctx context.Context, packageId string, version string, slug string, rulesetId string, docData string) (*view.IssuesSummary, error) {
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("enhancement_%s_%s_%s", packageId, version, slug))
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return nil, fmt.Errorf("enhancement lint: error creating temp directory: %s", err)
	}
	defer os.RemoveAll(tempDir)

	ext := ".json"           // TODO: Need to calc!!
	fileName := "file" + ext // Some linters (e.g. Spectral) have a problem with some characters is file names, so generating a safe one.
	filePath := filepath.Join(tempDir, fileName)
	if err := os.WriteFile(filePath, []byte(docData), 0600); err != nil {
		return nil, fmt.Errorf("error writing doc file: %s", err)
	}

	rs, err := e.ruleSetRepository.GetRulesetWithData(ctx, rulesetId)
	if err != nil {
		return nil, fmt.Errorf("enhancement lint: error getting ruleset: %s", err)
	}
	rsExt := filepath.Ext(rs.FileName)
	rulesetFileName := "ruleset" + rsExt // Some linters (e.g. Spectral) have a problem with some characters is file names, so generating a safe one.
	rulesetPath := filepath.Join(tempDir, rulesetFileName)
	if err = os.WriteFile(rulesetPath, rs.Data, 0600); err != nil {
		return nil, fmt.Errorf("enhancement lint: error writing ruleset file: %s", err)
	}

	status := view.StatusSuccess
	details := ""
	var result []byte
	var report []interface{}
	var summary view.SpectralResultSummary
	var sumAsMap map[string]interface{}

	// only spectral is supported now

	log.Infof("enhancement lint: Processing doc %s for package %s, version %s by spectral", slug, packageId, version)
	resultPath, _, err := e.spectralExecutor.LintLocalDoc(filePath, rulesetPath)
	if err != nil {
		status = view.StatusError
		details = fmt.Sprintf("error linting doc with spectral: %s", err)
	}

	if status == view.StatusSuccess {
		result, err = os.ReadFile(resultPath)
		if err != nil {
			status = view.StatusError
			details = fmt.Sprintf("error reading result file: %s", err)
		}
		log.Tracef("result file size is %d bytes", len(result))
	}

	if status == view.StatusSuccess {
		err = json.Unmarshal(result, &report)
		if err != nil {
			status = view.StatusError
			details = fmt.Sprintf("error unmarshalling result: %s", err)
		}
	}

	if status == view.StatusSuccess {
		summary = calculateSpectralSummary(report)

		sumJson, err := json.Marshal(summary)
		if err != nil {
			status = view.StatusError
			details = fmt.Sprintf("error marshaling summary: %s", err)
		} else {
			err = json.Unmarshal(sumJson, &sumAsMap)
			if err != nil {
				status = view.StatusError
				details = fmt.Sprintf("error unmarshaling summary: %s", err)
			}
		}
	}

	if status == view.StatusError {
		return nil, fmt.Errorf("enhancement lint: error: %s", details)
	}

	// TODO: save result to debug

	res := view.IssuesSummary{
		Error:   summary.ErrorCount,
		Warning: summary.WarningCount,
		Info:    summary.InfoCount,
		Hint:    summary.HintCount,
	}

	log.Infof("Lint result summary for enhanced file %s: %+v", slug, res)

	return &res, nil
}
