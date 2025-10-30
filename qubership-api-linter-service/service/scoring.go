package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/view"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"
)

type ScoringService interface {
	GenRestDocScore(ctx context.Context, task entity.DocumentLintTask, docData string, lintSummary view.SpectralResultSummary, lintReport []interface{}) (*view.Score, error)
	GetRestDocScoringData(ctx context.Context, packageId string, version string, slug string) (*view.Score, error)
	GenEnhancedRestDocScore(ctx context.Context, packageId string, version string, slug string, docData string, lintSummary view.IssuesSummary) (*view.Score, error)
	GetEnhancedRestDocScoringData(ctx context.Context, packageId string, version string, slug string) (*view.Score, error)
}

func NewScoringService(apihubClient client.ApihubClient, llmClient client.LLMClient, problemsService ProblemsService, localFileStore bool) ScoringService {
	storage := make(map[string]view.Score)
	enhancedStorage := make(map[string]view.Score)

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
	}

	return &scoringServiceImpl{
		apihubClient:    apihubClient,
		llmClient:       llmClient,
		problemsService: problemsService,
		localFileStore:  localFileStore,
		storage:         storage,
		enhancedStorage: enhancedStorage,
	}
}

type scoringServiceImpl struct {
	apihubClient    client.ApihubClient
	llmClient       client.LLMClient
	problemsService ProblemsService

	localFileStore  bool
	storage         map[string]view.Score
	enhancedStorage map[string]view.Score
}

func (s scoringServiceImpl) GetRestDocScoringData(ctx context.Context, packageId string, version string, slug string) (*view.Score, error) {
	ver, rev, err := getVersionAndRevision(ctx, s.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	key := packageId + "|" + fmt.Sprintf("%s@%d", ver, rev) + "|" + slug
	res := s.storage[key]

	if res.Details == nil {
		res.Details = []view.ScoreDetail{
			{
				Name:  "No data found",
				Value: "!",
			},
		}
	}
	return &res, nil
}

func (s scoringServiceImpl) GenRestDocScore(ctx context.Context, task entity.DocumentLintTask, docData string, lintSummary view.SpectralResultSummary, lintReport []interface{}) (*view.Score, error) {
	log.Infof("Run scoring for doc %s", task.FileId)
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

	problems, err := s.problemsService.GenTaskRestDocProblems(ctx, task.PackageId, task.Version, task.Revision, task.FileSlug, docData)
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
		err = saveDebugData(task, docData, lintSummary, lintReport, problems, "")
		if err != nil {
			return nil, err
		}
	}

	// TODO: bwc problems??

	key := task.PackageId + "|" + fmt.Sprintf("%s@%d", task.Version, task.Revision) + "|" + task.FileSlug

	s.storage[key] = result
	s.saveStorage()

	return &result, nil
}

func saveDebugData(task entity.DocumentLintTask, docData string, lintSummary view.SpectralResultSummary, lintReport []interface{}, problems []view.AIApiDocCatProblem, fixedDocs string) error {
	// Create directory name using current date
	currentTime := time.Now()
	dirName := currentTime.Format("2006-01-02_15_03_04")

	// Create directory (with 0755 permissions)
	if err := os.MkdirAll(dirName, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Save task data
	taskData, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirName, "task.json"), taskData, 0644); err != nil {
		return fmt.Errorf("failed to write task file: %v", err)
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

	// Save lint report
	reportData, err := json.MarshalIndent(lintReport, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lint report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirName, "lintReport.json"), reportData, 0644); err != nil {
		return fmt.Errorf("failed to write lintReport file: %v", err)
	}

	// Save problems
	problemsData, err := json.MarshalIndent(problems, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal problems: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirName, "problems.json"), problemsData, 0644); err != nil {
		return fmt.Errorf("failed to write problems file: %v", err)
	}

	// Save fixed documents
	if err := os.WriteFile(filepath.Join(dirName, "fixedDocs.txt"), []byte(fixedDocs), 0644); err != nil {
		return fmt.Errorf("failed to write fixedDocs file: %v", err)
	}

	return nil
}

func (s scoringServiceImpl) saveStorage() {
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
}

func (s scoringServiceImpl) GenEnhancedRestDocScore(ctx context.Context, packageId string, version string, slug string, docData string, lintSummary view.IssuesSummary) (*view.Score, error) {
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

func (s scoringServiceImpl) GetEnhancedRestDocScoringData(ctx context.Context, packageId string, version string, slug string) (*view.Score, error) {
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
