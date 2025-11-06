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
}

func NewScoringService(apihubClient client.ApihubClient, llmClient client.LLMClient, problemsService ProblemsService, localFileStore bool) ScoringService {
	storage := make(map[string]view.Score)
	enhancedStorage := make(map[string]view.Score)
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
		apihubClient:    apihubClient,
		llmClient:       llmClient,
		problemsService: problemsService,
		localFileStore:  localFileStore,
		statusStorage:   statusStorage,
		storage:         storage,
		enhancedStorage: enhancedStorage,
		mutex:           sync.RWMutex{},
	}
}

type scoringServiceImpl struct {
	apihubClient    client.ApihubClient
	llmClient       client.LLMClient
	problemsService ProblemsService

	localFileStore  bool
	statusStorage   map[string]view.EnhancementStatusResponse
	storage         map[string]view.Score
	enhancedStorage map[string]view.Score
	mutex           sync.RWMutex
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
				log.Errorf("No lint result for doc " + doc.Slug)
				s.mutex.Lock()
				s.statusStorage[key] = view.EnhancementStatusResponse{
					Status:  view.ESError,
					Details: "No lint result for doc " + doc.Slug,
				}
				s.mutex.Unlock()
				return
			}
			data, err := s.apihubClient.GetDocumentRawData(asyncCtx, packageId, version, doc.Slug)
			if err != nil {
				log.Errorf("get raw doc: " + err.Error())
				s.mutex.Lock()
				s.statusStorage[key] = view.EnhancementStatusResponse{
					Status:  view.ESError,
					Details: "get raw doc: " + err.Error(),
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
