package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/utils"
	"github.com/Netcracker/qubership-api-linter-service/view"
	log "github.com/sirupsen/logrus"
)

type ScoringService interface {
	MakeRestDocScore(ctx context.Context, packageId string, version string, slug string, docData string, lintSummary view.SpectralResultSummary) (*view.Score, error)
	GetRestDocScoringData(ctx context.Context, packageId string, version string, slug string) (*view.Score, error)

	StartMakeVersionScore(ctx context.Context, packageId string, version string, lintSummary view.ValidationSummaryForVersion) error
	GetRestDocScoringStatus(ctx context.Context, packageId string, version string, slug string) (view.EnhancementStatusResponse, error)

	MakeEnhancedRestDocScore(ctx context.Context, packageId string, version string, slug string, docData string, lintSummary view.IssuesSummary) (*view.Score, error)
	GetEnhancedRestDocScoringData(ctx context.Context, packageId string, version string, slug string) (*view.Score, error)

	MakeRestOpScore(ctx context.Context, packageId string, version string, slug string, operationId string, opData string, lintSummary view.SpectralResultSummary) (*view.Score, error)
	GetRestOpScoringData(ctx context.Context, packageId string, version string, slug string, operationId string) (*view.Score, error)

	GetVersionScore(ctx context.Context, packageId string, version string) (*view.Score, error)
}

func NewScoringService(apihubClient client.ApihubClient, llmClient client.LLMClient, problemsService ProblemsService, operationResultRepository repository.OperationResultRepository, scoringRepository repository.ScoringRepository, localFileStore bool) ScoringService {
	storage := make(map[string]view.Score)
	enhancedStorage := make(map[string]view.Score)
	operationStorage := make(map[string]view.Score)
	statusStorage := make(map[string]view.EnhancementStatusResponse)
	if localFileStore {
		data, err := os.ReadFile("scoring_storage.json")
		if err == nil {
			if err := json.Unmarshal(data, &storage); err != nil {
				log.Errorf("Warning: Failed to unmarshal storage file: %v", err)
			}
		} else {
			log.Warnf("Warning: Failed to read storage file: %v", err)
		}

		data, err = os.ReadFile("scoring_enhanced_storage.json")
		if err == nil {
			if err := json.Unmarshal(data, &enhancedStorage); err != nil {
				log.Errorf("Warning: Failed to unmarshal storage file: %v", err)
			}
		} else {
			log.Warnf("Warning: Failed to read storage file: %v", err)
		}

		data, err = os.ReadFile("scoring_operation_storage.json")
		if err == nil {
			if err := json.Unmarshal(data, &operationStorage); err != nil {
				log.Errorf("Warning: Failed to unmarshal operation storage file: %v", err)
			}
		} else {
			log.Warnf("Warning: Failed to read operation storage file: %v", err)
		}

		data, err = os.ReadFile("scoring_status_storage.json")
		if err == nil {
			if err := json.Unmarshal(data, &statusStorage); err != nil {
				log.Errorf("Warning: Failed to unmarshal storage file: %v", err)
			}
		} else {
			log.Warnf("Warning: Failed to read storage file: %v", err)
		}
	}

	return &scoringServiceImpl{
		apihubClient:              apihubClient,
		llmClient:                 llmClient,
		problemsService:           problemsService,
		operationResultRepository: operationResultRepository,
		scoringRepository:         scoringRepository,
		localFileStore:            localFileStore,
		statusStorage:             statusStorage,
		storage:                   storage,
		enhancedStorage:           enhancedStorage,
		operationStorage:          operationStorage,
		mutex:                     sync.RWMutex{},
	}
}

type scoringServiceImpl struct {
	apihubClient              client.ApihubClient
	llmClient                 client.LLMClient
	problemsService           ProblemsService
	operationResultRepository repository.OperationResultRepository
	scoringRepository         repository.ScoringRepository

	localFileStore   bool
	statusStorage    map[string]view.EnhancementStatusResponse
	storage          map[string]view.Score
	enhancedStorage  map[string]view.Score
	operationStorage map[string]view.Score
	mutex            sync.RWMutex
}

func (s *scoringServiceImpl) GetVersionScore(ctx context.Context, packageId string, version string) (*view.Score, error) {
	// use only operation scores

	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	// Get all operations for the version
	operations, err := s.operationResultRepository.GetOperationsForVersion(ctx, packageId, ver)
	if err != nil {
		return nil, err
	}

	// Collect DigitalScore values and Details from operation scores
	var digitalScores []int
	var allDetails []view.ScoreDetail
	for _, op := range operations {
		// Filter by revision to get only operations for the specific revision
		if op.Revision != rev {
			continue
		}

		// Try to get from database first
		ent, err := s.scoringRepository.GetScore(ctx, packageId, op.Version, op.Revision, op.OperationId)
		if err != nil {
			log.Warnf("Failed to get score from database for operation %s: %v", op.OperationId, err)
		} else if ent.Score.DigitalScore > 0 || len(ent.Score.Details) > 0 {
			if ent.Score.DigitalScore > 0 {
				digitalScores = append(digitalScores, ent.Score.DigitalScore)
			}
			if len(ent.Score.Details) > 0 {
				allDetails = append(allDetails, ent.Score.Details...)
			}
			continue
		}
	}

	// Calculate average DigitalScore
	var result view.Score
	if len(digitalScores) == 0 {
		result.DigitalScore = 0
	} else {
		sum := 0
		for _, score := range digitalScores {
			sum += score
		}
		result.DigitalScore = sum / len(digitalScores)
	}

	// Aggregate Details: take worst grade for each ScoreName
	// Grade priority: Bad > Acceptable > Good
	detailMap := make(map[view.ScoreName]view.Grade)
	for _, detail := range allDetails {
		currentGrade, exists := detailMap[detail.Name]
		if !exists {
			detailMap[detail.Name] = detail.Value
		} else {
			// Update to worst grade
			if detail.Value == view.Bad {
				detailMap[detail.Name] = view.Bad
			} else if detail.Value == view.Acceptable && currentGrade == view.Good {
				detailMap[detail.Name] = view.Acceptable
			}
		}
	}

	// Convert map back to slice
	result.Details = make([]view.ScoreDetail, 0, len(detailMap))
	for name, grade := range detailMap {
		result.Details = append(result.Details, view.ScoreDetail{
			Name:  name,
			Value: grade,
		})
	}

	return &result, nil
}

func (s *scoringServiceImpl) StartMakeVersionScore(ctx context.Context, packageId string, version string, lintSummary view.ValidationSummaryForVersion) error {
	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return err
	}

	version = fmt.Sprintf("%s@%d", ver, rev)
	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.statusStorage[key] = view.EnhancementStatusResponse{
		Status:  view.ESProcessing,
		Details: "",
	}

	utils.SafeAsync(func() {
		log.Infof("Start manual scoring for %s version %s", packageId, version)
		defer log.Infof("Finished manual scoring for %s version %s", packageId, version)
		asyncCtx := secctx.MakeSysadminContext(context.Background())
		for _, doc := range lintSummary.Documents {
			if doc.IssuesSummary == nil {
				log.Errorf("No lint result for doc %s", doc.Slug)
				s.mutex.Lock()
				s.statusStorage[key] = view.EnhancementStatusResponse{
					Status:  view.ESError,
					Details: fmt.Sprintf("No lint result for doc %s", doc.Slug),
				}
				s.mutex.Unlock()
				return
			}
			data, err := s.apihubClient.GetDocumentRawData(asyncCtx, packageId, version, doc.Slug)
			if err != nil {
				log.Errorf("get raw doc: %v", err)
				s.mutex.Lock()
				s.statusStorage[key] = view.EnhancementStatusResponse{
					Status:  view.ESError,
					Details: fmt.Sprintf("get raw doc: %v", err),
				}
				s.mutex.Unlock()
				return
			}

			convSumm := view.SpectralResultSummary{
				ErrorCount:   doc.IssuesSummary.Error,
				WarningCount: doc.IssuesSummary.Warning,
				InfoCount:    doc.IssuesSummary.Info,
				HintCount:    doc.IssuesSummary.Hint,
			}
			_, err = s.MakeRestDocScore(asyncCtx, packageId, version, doc.Slug, string(data), convSumm)
			if err != nil {
				log.Errorf("Failed to make async rest doc score: %v", err)
				s.mutex.Lock()
				s.statusStorage[key] = view.EnhancementStatusResponse{
					Status:  view.ESError,
					Details: err.Error(),
				}
				s.mutex.Unlock()
				return
			}
			s.scoreOperationsForDocument(asyncCtx, packageId, version, doc)
		}
		s.mutex.Lock()
		defer s.mutex.Unlock()
		s.statusStorage[key] = view.EnhancementStatusResponse{
			Status:  view.ESSuccess,
			Details: "",
		}
	})
	return nil
}

func (s *scoringServiceImpl) GetRestDocScoringStatus(ctx context.Context, packageId string, version string, slug string) (view.EnhancementStatusResponse, error) {
	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return view.EnhancementStatusResponse{
			Status:  view.ESError,
			Details: err.Error(),
		}, err
	}

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev)
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	res, exists := s.statusStorage[key]
	if !exists {
		return view.EnhancementStatusResponse{
			Status:  view.ESNotStarted,
			Details: "",
		}, nil
	}
	return res, nil
}

func (s *scoringServiceImpl) GetRestDocScoringData(ctx context.Context, packageId string, version string, slug string) (*view.Score, error) {
	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug
	res := s.storage[key]

	if res.Details == nil {
		res.Details = []view.ScoreDetail{}
	}
	return &res, nil
}

func (s *scoringServiceImpl) MakeRestDocScore(ctx context.Context, packageId string, version string, slug string, docData string, lintSummary view.SpectralResultSummary) (*view.Score, error) {
	log.Infof("Run scoring for doc %s", slug)

	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	var result view.Score

	lintGrade := view.Good
	if lintSummary.ErrorCount > 0 {
		lintGrade = view.Bad
	}
	if lintSummary.WarningCount > 0 && lintGrade == view.Good {
		lintGrade = view.Acceptable
	}

	result.Details = append(result.Details, view.ScoreDetail{
		Name:  view.ScoreNameLint,
		Value: lintGrade,
	})

	problems, err := s.problemsService.GenTaskRestDocProblems(ctx, packageId, ver, rev, slug, docData)
	if err != nil {
		return nil, err
	}

	problGrade := view.Good
	for _, problem := range problems {
		if problem.Severity == "error" {
			problGrade = view.Bad
		}
		if problem.Severity == "warning" && problGrade == view.Good {
			problGrade = view.Acceptable
		}
	}
	result.Details = append(result.Details, view.ScoreDetail{
		Name:  view.ScoreNameProblems,
		Value: problGrade,
	})

	totalGrade := view.Good
	if lintGrade == view.Acceptable || problGrade == view.Acceptable {
		totalGrade = view.Acceptable
	}
	if lintGrade == view.Bad || problGrade == view.Bad {
		totalGrade = view.Bad
	}

	result.OverallScore = totalGrade

	if s.localFileStore {
		err = saveDebugData(docData, lintSummary, problems)
		if err != nil {
			return nil, err
		}
	}

	// TODO: bwc problems??

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug

	s.storage[key] = result
	s.saveStorage()

	return &result, nil
}

func saveDebugData(docData string, lintSummary view.SpectralResultSummary, problems []view.AIApiDocCatProblem) error {
	// Create directory name using current date
	currentTime := time.Now()
	dirName := currentTime.Format("2006-01-02_15_03_04")

	// Create directory (with 0755 permissions)
	if err := os.MkdirAll(dirName, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Save document data
	if err := os.WriteFile(filepath.Join(dirName, "docData.txt"), []byte(docData), 0644); err != nil {
		return fmt.Errorf("failed to write docData file: %v", err)
	}

	// Save lint summary
	summaryData, err := json.MarshalIndent(lintSummary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lint summary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirName, "lintSummary.json"), summaryData, 0644); err != nil {
		return fmt.Errorf("failed to write lintSummary file: %v", err)
	}

	// Save problems
	problemsData, err := json.MarshalIndent(problems, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal problems: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirName, "problems.json"), problemsData, 0644); err != nil {
		return fmt.Errorf("failed to write problems file: %v", err)
	}

	return nil
}

func (s *scoringServiceImpl) saveStorage() {
	if !s.localFileStore {
		return
	}
	data, err := json.Marshal(s.storage)
	if err != nil {
		log.Errorf("err: %+v", err)
		return
	}

	err = os.WriteFile("scoring_storage.json", data, 0644)
	if err != nil {
		log.Errorf("Failed to save storage to file: %+v", err)
	}

	enhData, err := json.Marshal(s.enhancedStorage)
	if err != nil {
		log.Errorf("err: %+v", err)
		return
	}

	err = os.WriteFile("scoring_enhanced_storage.json", enhData, 0644)
	if err != nil {
		log.Errorf("Failed to save enh storage to file: %+v", err)
	}

	opData, err := json.Marshal(s.operationStorage)
	if err != nil {
		log.Errorf("err: %+v", err)
		return
	}

	err = os.WriteFile("scoring_operation_storage.json", opData, 0644)
	if err != nil {
		log.Errorf("Failed to save operation storage to file: %+v", err)
	}

	statusData, err := json.Marshal(s.statusStorage)
	if err != nil {
		log.Errorf("err: %+v", err)
		return
	}

	err = os.WriteFile("scoring_status_storage.json", statusData, 0644)
	if err != nil {
		log.Errorf("Failed to save status storage to file: %+v", err)
	}
}

func (s *scoringServiceImpl) MakeEnhancedRestDocScore(ctx context.Context, packageId string, version string, slug string, docData string, lintSummary view.IssuesSummary) (*view.Score, error) {
	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	log.Infof("Run scoring for doc %s", slug)
	var result view.Score

	lintGrade := view.Good
	if lintSummary.Error > 0 {
		lintGrade = view.Bad
	}
	if lintSummary.Warning > 0 && lintGrade == view.Good {
		lintGrade = view.Acceptable
	}

	result.Details = append(result.Details, view.ScoreDetail{
		Name:  view.ScoreNameLint,
		Value: lintGrade,
	})

	problems, err := s.problemsService.GenTaskRestDocProblems(ctx, packageId, ver, rev, slug, docData)
	if err != nil {
		return nil, err
	}

	problGrade := view.Good
	for _, problem := range problems {
		if problem.Severity == "error" {
			problGrade = view.Bad
		}
		if problem.Severity == "warning" && problGrade == view.Good {
			problGrade = view.Acceptable
		}
	}
	result.Details = append(result.Details, view.ScoreDetail{
		Name:  view.ScoreNameProblems,
		Value: problGrade,
	})

	totalGrade := view.Good
	if lintGrade == view.Acceptable || problGrade == view.Acceptable {
		totalGrade = view.Acceptable
	}
	if lintGrade == view.Bad || problGrade == view.Bad {
		totalGrade = view.Bad
	}

	result.OverallScore = totalGrade

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug

	s.enhancedStorage[key] = result
	s.saveStorage()

	return &result, nil
}

func (s *scoringServiceImpl) GetEnhancedRestDocScoringData(ctx context.Context, packageId string, version string, slug string) (*view.Score, error) {
	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug
	res := s.enhancedStorage[key]
	if res.Details == nil {
		res.Details = make([]view.ScoreDetail, 0)
	}
	return &res, nil
}

func (s *scoringServiceImpl) scoreOperationsForDocument(ctx context.Context, packageId string, version string, doc view.ValidationDocument) {
	if doc.RulesetId == "" {
		log.Warnf("Doc %s has empty ruleset id; skip operation scoring", doc.Slug)
		return
	}

	docDetails, err := s.apihubClient.GetDocumentDetails(ctx, packageId, version, doc.Slug)
	if err != nil {
		log.Warnf("Failed to get document details for doc %s: %v", doc.Slug, err)
		return
	}
	if docDetails == nil {
		log.Warnf("No document details returned for doc %s; skip operation scoring", doc.Slug)
		return
	}

	for _, op := range docDetails.Operations {
		operationWithData, err := s.apihubClient.GetOperationWithData(ctx, packageId, version, op.ApiType, op.OperationId)
		if err != nil {
			log.Warnf("Failed to get data for operation %s (doc %s): %v", op.OperationId, doc.Slug, err)
			continue
		}
		if operationWithData == nil {
			log.Warnf("Operation %s (doc %s) not found while scoring", op.OperationId, doc.Slug)
			continue
		}

		opHash := op.DataHash
		if opHash == "" {
			opHash = utils.CreateSHA256Hash(operationWithData.Data)
		}
		if opHash == "" {
			log.Warnf("Operation %s (doc %s) has empty data hash; skip scoring", op.OperationId, doc.Slug)
			continue
		}

		operationResult, err := s.operationResultRepository.GetOperationResult(ctx, opHash, doc.RulesetId)
		if err != nil {
			log.Warnf("Failed to get lint summary for operation %s (doc %s): %v", op.OperationId, doc.Slug, err)
			continue
		}
		if operationResult == nil || operationResult.Summary == nil {
			log.Warnf("Lint summary is missing for operation %s (doc %s)", op.OperationId, doc.Slug)
			continue
		}

		spectralSummary, err := spectralSummaryFromMap(operationResult.Summary)
		if err != nil {
			log.Warnf("Failed to convert lint summary for operation %s (doc %s): %v", op.OperationId, doc.Slug, err)
			continue
		}

		_, err = s.MakeRestOpScore(ctx, packageId, version, doc.Slug, op.OperationId, string(operationWithData.Data), spectralSummary)
		if err != nil {
			log.Warnf("Failed to generate score for operation %s (doc %s): %v", op.OperationId, doc.Slug, err)
		}
	}
}

func spectralSummaryFromMap(summary map[string]interface{}) (view.SpectralResultSummary, error) {
	if summary == nil {
		return view.SpectralResultSummary{}, fmt.Errorf("summary map is nil")
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return view.SpectralResultSummary{}, err
	}
	var result view.SpectralResultSummary
	if err := json.Unmarshal(data, &result); err != nil {
		return view.SpectralResultSummary{}, err
	}
	return result, nil
}

func (s *scoringServiceImpl) MakeRestOpScore(ctx context.Context, packageId string, version string, slug string, operationId string, opData string, lintSummary view.SpectralResultSummary) (*view.Score, error) {
	log.Infof("Run scoring for operation %s in doc %s", operationId, slug)

	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	var result view.Score

	lintGrade := view.Good
	if lintSummary.ErrorCount > 0 {
		lintGrade = view.Bad
	}
	if lintSummary.WarningCount > 0 && lintGrade == view.Good {
		lintGrade = view.Acceptable
	}

	result.Details = append(result.Details, view.ScoreDetail{
		Name:  view.ScoreNameLint,
		Value: lintGrade,
	})

	problems, err := s.problemsService.GenTaskRestOpProblems(ctx, packageId, ver, rev, slug, operationId, opData)
	if err != nil {
		return nil, err
	}

	score := CalculateScore(lintSummary, problems)
	result.DigitalScore = score

	problGrade := view.Good
	for _, problem := range problems {
		if problem.Severity == "error" {
			problGrade = view.Bad
		}
		if problem.Severity == "warning" && problGrade == view.Good {
			problGrade = view.Acceptable
		}
	}
	result.Details = append(result.Details, view.ScoreDetail{
		Name:  view.ScoreNameProblems,
		Value: problGrade,
	})

	totalGrade := view.Good
	if lintGrade == view.Acceptable || problGrade == view.Acceptable {
		totalGrade = view.Acceptable
	}
	if lintGrade == view.Bad || problGrade == view.Bad {
		totalGrade = view.Bad
	}

	result.OverallScore = totalGrade

	if s.localFileStore {
		err = saveDebugData(opData, lintSummary, problems)
		if err != nil {
			return nil, err
		}
	}

	// Save to database
	ent := entity.OperationScore{
		PackageId:   packageId,
		Version:     ver,
		Revision:    rev,
		OperationId: operationId,
		Score:       result,
	}
	err = s.scoringRepository.SaveScore(ctx, ent)
	if err != nil {
		log.Errorf("Failed to save score to database: %v", err)
		// Continue execution even if save fails
	}

	// Keep backward compatibility with local file store
	if s.localFileStore {
		key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug + "|" + operationId
		s.mutex.Lock()
		s.operationStorage[key] = result
		s.mutex.Unlock()
		s.saveStorage()
	}

	return &result, nil
}

func (s *scoringServiceImpl) GetRestOpScoringData(ctx context.Context, packageId string, version string, slug string, operationId string) (*view.Score, error) {
	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	// Try to get from database first
	ent, err := s.scoringRepository.GetScore(ctx, packageId, ver, rev, operationId)
	if err != nil {
		log.Warnf("Failed to get score from database: %v", err)
	} else if ent.Score.Details != nil || ent.Score.DigitalScore > 0 {
		if ent.Score.Details == nil {
			ent.Score.Details = []view.ScoreDetail{}
		}
		return &ent.Score, nil
	}

	res := view.Score{
		Details: []view.ScoreDetail{},
	}
	return &res, nil
}
