package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/utils"
	"github.com/Netcracker/qubership-api-linter-service/view"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

type DocTaskProcessor interface {
	Start()
}

func NewDocTaskProcessor(docTaskRepo repository.DocLintTaskRepository, ruleSetRepository repository.RulesetRepository,
	docResultRepository repository.DocResultRepository, lintResultRepository repository.LintResultRepository,
	cl client.ApihubClient, spectralExecutor SpectralExecutor, aiOasExecutor AiOasExecutor, executorId string, spectralLinterWorkers, aiLinterWorkers int) DocTaskProcessor {
	return &docTaskProcessorImpl{
		docTaskRepo:           docTaskRepo,
		ruleSetRepository:     ruleSetRepository,
		docResultRepository:   docResultRepository,
		lintResultRepository:  lintResultRepository,
		cl:                    cl,
		spectralExecutor:      spectralExecutor,
		aiOasExecutor:         aiOasExecutor,
		executorId:            executorId,
		spectralLinterWorkers: spectralLinterWorkers,
		aiLinterWorkers:       aiLinterWorkers,
	}
}

type docTaskProcessorImpl struct {
	docTaskRepo          repository.DocLintTaskRepository
	ruleSetRepository    repository.RulesetRepository
	docResultRepository  repository.DocResultRepository
	lintResultRepository repository.LintResultRepository
	cl                   client.ApihubClient
	spectralExecutor     SpectralExecutor
	aiOasExecutor        AiOasExecutor

	executorId            string
	spectralLinterWorkers int
	aiLinterWorkers       int
}

func (d docTaskProcessorImpl) Start() {
	for i := 0; i < d.spectralLinterWorkers; i++ {
		workerId := i
		utils.SafeAsync(func() {
			d.runWorkerLoop(view.SpectralLinter, view.SpectralAsyncLinter)
			log.Tracef("docTaskProcessorImpl: Spectral worker %d exited", workerId)
		})
	}

	for i := 0; i < d.aiLinterWorkers; i++ {
		workerId := i
		utils.SafeAsync(func() {
			d.runWorkerLoop(view.AiOasLinter)
			log.Tracef("docTaskProcessorImpl: AI worker %d exited", workerId)
		})
	}
	log.Infof("docTaskProcessorImpl: started %d Spectral and %d AI linter workers", d.spectralLinterWorkers, d.aiLinterWorkers)
}

func (d docTaskProcessorImpl) runWorkerLoop(linters ...view.Linter) {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	running := atomic.Bool{}
	for range ticker.C {
		if running.Load() {
			log.Tracef("docTaskProcessorImpl: worker skipped, still running")
			continue
		}

		utils.SafeAsync(func() {
			running.Store(true)
			for {
				moreWork := d.processTask(linters...)
				if !moreWork {
					break
				}
				log.Tracef("docTaskProcessorImpl: keep on running")
			}
			running.Store(false)
		})
	}
}

func (d docTaskProcessorImpl) processTask(linters ...view.Linter) bool {
	task, err := d.docTaskRepo.FindFreeDocTask(context.Background(), d.executorId, linters...)
	if err != nil {
		log.Errorf("Error finding free doc task: %s", err)
		return false
	}
	if task != nil {
		d.processDocTask(secctx.MakeSysadminContext(context.Background()), *task)
		d.writeAsyncTestLog(task.Id)
		return true
	}
	return false
}

func (d docTaskProcessorImpl) handleError(ctx context.Context, task entity.DocumentLintTask, err error, lintTimeMs int64) {
	log.Infof("Doc task %s failed with error: %s", task.Id, err)

	docEnt := entity.LintedDocument{
		PackageId:         task.PackageId,
		Version:           task.Version,
		Revision:          task.Revision,
		Slug:              task.FileSlug,
		FileId:            task.FileId,
		SpecificationType: task.APIType,
		RulesetId:         task.RulesetId,
		DataHash:          "", // set to empty string because in some error cases it is not available
		LintStatus:        view.StatusError,
		LintDetails:       err.Error(),
	}

	verEnt := entity.LintedVersion{
		PackageId:   task.PackageId,
		Version:     task.Version,
		Revision:    task.Revision,
		LintStatus:  view.VersionStatusInProgress,
		LintDetails: "",
		LintedAt:    time.Now(),
	}

	err = d.docResultRepository.SaveLintResult(ctx, task.Id, view.StatusError, err.Error(),
		lintTimeMs, verEnt, docEnt, nil, d.executorId)
	if err != nil {
		log.Errorf("Handle error for doc task %s failed: unable to save lint result: %s", task.Id, err)
	}
}

func (d docTaskProcessorImpl) processDocTask(ctx context.Context, task entity.DocumentLintTask) {
	start := time.Now()

	runningC := make(chan struct{})
	defer func() {
		close(runningC)
	}()

	// Update last_active during long run
	utils.SafeAsync(func() {
		t := time.NewTicker(time.Second * 5)
		select {
		case <-ctx.Done():
			t.Stop()
			break
		case _, ok := <-t.C:
			if !ok {
				t.Stop()
				break
			}
			err := d.docTaskRepo.SetDocTaskStatus(ctx, task.Id, view.TaskStatusProcessing, "", d.executorId)
			if err != nil {
				log.Errorf("Error updating status of doc task %s: %s", task.Id, err)
			}
		case _, ok := <-runningC:
			if !ok {
				t.Stop()
				break
			}
		}
	})

	data, err := d.cl.GetDocumentRawData(ctx, task.PackageId, fmt.Sprintf("%s@%d", task.Version, task.Revision), task.FileSlug)
	if err != nil {
		d.handleError(ctx, task, err, time.Since(start).Milliseconds())
		return
	}

	if len(data) == 0 {
		d.handleError(ctx, task, fmt.Errorf("document data is empty"), time.Since(start).Milliseconds())
		return
	}

	docHash := utils.CreateSHA256Hash(data)

	// Validation shortcut: reuse cached result if document (by hash) + ruleset was already linted with same linter version
	currentLinterVersion := d.getLinterVersion(task.Linter)
	cached, err := d.lintResultRepository.GetLintResult(ctx, docHash, task.RulesetId)
	if err != nil {
		log.Warnf("Failed to check lint cache for task %s: %s", task.Id, err)
	}
	if cached != nil && cached.LinterVersion == currentLinterVersion {
		log.Infof("Using cached lint result for doc %s (task id = %s), hash = %s", task.FileId, task.Id, docHash)
		d.saveLintResultFromCache(ctx, task, docHash, cached, time.Since(start).Milliseconds())
		return
	}

	tempDir := filepath.Join(os.TempDir(), task.Id)
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		d.handleError(ctx, task, fmt.Errorf("error creating temp directory: %s", err), time.Since(start).Milliseconds())
		return
	}
	defer os.RemoveAll(tempDir)
	ext := filepath.Ext(task.FileId)
	fileName := "file" + ext // Some linters (e.g. Spectral) have a problem with some characters is file names, so generating a safe one.
	filePath := filepath.Join(tempDir, fileName)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		d.handleError(ctx, task, fmt.Errorf("error writing doc file: %s", err), time.Since(start).Milliseconds())
		return
	}

	rs, err := d.ruleSetRepository.GetRulesetWithData(ctx, task.RulesetId)
	if err != nil {
		d.handleError(ctx, task, fmt.Errorf("error getting ruleset: %s", err), time.Since(start).Milliseconds())
		return
	}
	rsExt := filepath.Ext(rs.FileName)
	rulesetFileName := "ruleset" + rsExt // Some linters (e.g. Spectral) have a problem with some characters is file names, so generating a safe one.
	rulesetPath := filepath.Join(tempDir, rulesetFileName)
	if err := os.WriteFile(rulesetPath, rs.Data, 0600); err != nil {
		d.handleError(ctx, task, fmt.Errorf("error writing ruleset file: %s", err), time.Since(start).Milliseconds())
		return
	}

	status := view.StatusSuccess
	details := ""
	var result []byte
	var report []interface{}
	var summary view.SpectralResultSummary
	var sumAsMap map[string]interface{}
	var LinterVersion string
	var calcTimeMs int64
	logDetails := ""

	if task.Linter == view.SpectralLinter || task.Linter == view.SpectralAsyncLinter {
		// TODO: move prepare file here?

		// it might take a long time due to linter lock or just long execution
		var resultPath string

		log.Infof("Processing by spectral doc %s (task id = %s) for package %s, version %s@%d", task.FileId, task.Id, task.PackageId, task.Version, task.Revision)
		resultPath, calcTimeMs, err = d.spectralExecutor.LintLocalDoc(filePath, rulesetPath)
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

		if details != "" {
			logDetails = fmt.Sprintf("details = %s, ", details)
		}

		LinterVersion = d.spectralExecutor.GetLinterVersion()
		log.Tracef("Spectral linter version is %s", LinterVersion)
	}

	if task.Linter == view.AiOasLinter {
		doc, err := d.cl.GetDocumentDetails(ctx, task.PackageId, task.Version, task.FileSlug)
		if err != nil {
			status = view.StatusError
			details = fmt.Sprintf("error getting document details: %s", err)
		}

		var rulesetData view.AiRuleset

		if status == view.StatusSuccess {
			err = yaml.Unmarshal(rs.Data, &rulesetData)
			if err != nil {
				status = view.StatusError
				details = fmt.Sprintf("failed to unmarshal AI ruleset: %s", err)
			}
		}

		var issues []view.ValidationIssue
		issueKeys := map[string]struct{}{}

		if status == view.StatusSuccess {
			log.Infof("Processing by AI linter doc %s (task id = %s) operations count = %d for package %s, version %s@%d", task.FileId, task.Id, len(doc.Operations), task.PackageId, task.Version, task.Revision)

			var mu sync.Mutex
			g, gCtx := errgroup.WithContext(ctx)
			for _, op := range doc.Operations {
				op := op
				g.Go(func() error {
					operation, err := d.cl.GetOperationWithData(gCtx, task.PackageId, task.Version, view.RestApiType, op.OperationId)
					if err != nil {
						return fmt.Errorf("error getting document details: %w", err)
					}

					opIssues, opCalcTime, err := d.aiOasExecutor.LintOperationsForDoc(gCtx, string(operation.Data), rulesetData)
					if err != nil {
						return fmt.Errorf("error linting doc with AI: %w", err)
					}

					mu.Lock()
					for _, opIssue := range opIssues {
						key := strings.Join(opIssue.Path, ".") + "|" + opIssue.Message // TODO any other data?
						if _, exists := issueKeys[key]; exists {
							log.Infof("Excluded as duplicate issue with path %+v and message = '%s' ", opIssue.Path, opIssue.Message)
							continue
						}
						issueKeys[key] = struct{}{}
						issues = append(issues, opIssue)
					}
					calcTimeMs += opCalcTime
					mu.Unlock()
					return nil
				})
			}

			if err := g.Wait(); err != nil {
				status = view.StatusError
				details = err.Error()
			}
		}

		if status == view.StatusSuccess {
			for _, issue := range issues {
				switch issue.Severity {
				case "error":
					summary.ErrorCount += 1
				case "warning":
					summary.WarningCount += 1
				case "info":
					summary.InfoCount += 1
				case "hint":
					summary.HintCount += 1
				}
			}

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

		if details != "" {
			logDetails = fmt.Sprintf("details = %s, ", details)
		}

		LinterVersion = d.aiOasExecutor.GetLinterVersion()

		issuesB, err := json.Marshal(issues)
		if err != nil {
			status = view.StatusError
			details = fmt.Sprintf("error marshaling issues: %s", err)
		}

		result = issuesB
	}

	if LinterVersion != "" { // if lint was performed
		log.Infof("%s lint finished for doc %s (task id = %s), status = %s, %sProcessing time = %+vms", task.Linter, task.FileId, task.Id, status, logDetails, calcTimeMs)

		log.Tracef("%s linter version is %s", task.Linter, LinterVersion)

		docEnt := entity.LintedDocument{
			PackageId:         task.PackageId,
			Version:           task.Version,
			Revision:          task.Revision,
			Slug:              task.FileSlug,
			FileId:            task.FileId,
			SpecificationType: task.APIType,
			RulesetId:         task.RulesetId,
			DataHash:          docHash,
			LintStatus:        status,
			LintDetails:       details,
		}

		verEnt := entity.LintedVersion{
			PackageId:   task.PackageId,
			Version:     task.Version,
			Revision:    task.Revision,
			LintStatus:  view.VersionStatusInProgress,
			LintDetails: "",
			LintedAt:    time.Now(),
		}

		var lintFileResult *entity.LintFileResult

		if status == view.StatusSuccess {
			lintFileResult = &entity.LintFileResult{
				DataHash:      docHash,
				RulesetId:     task.RulesetId,
				LinterVersion: LinterVersion,
				Data:          result,
				Summary:       sumAsMap,
			}
		}

		err = d.docResultRepository.SaveLintResult(context.Background(), task.Id, status, details, calcTimeMs, verEnt, docEnt, lintFileResult, d.executorId)
		if err != nil {
			d.handleError(ctx, task, fmt.Errorf("failed to save lint result with error: %s", err), time.Since(start).Milliseconds())
			return
		}
	} else {
		d.handleError(ctx, task, fmt.Errorf("selected linter %s is not supported", task.Linter), time.Since(start).Milliseconds())
		return
	}
}

func (d docTaskProcessorImpl) getLinterVersion(linter view.Linter) string {
	switch linter {
	case view.SpectralLinter, view.SpectralAsyncLinter:
		return d.spectralExecutor.GetLinterVersion()
	case view.AiOasLinter:
		return d.aiOasExecutor.GetLinterVersion()
	default:
		return ""
	}
}

func (d docTaskProcessorImpl) saveLintResultFromCache(ctx context.Context, task entity.DocumentLintTask, docHash string, cached *entity.LintFileResult, lintTimeMs int64) {
	docEnt := entity.LintedDocument{
		PackageId:         task.PackageId,
		Version:           task.Version,
		Revision:          task.Revision,
		Slug:              task.FileSlug,
		FileId:            task.FileId,
		SpecificationType: task.APIType,
		RulesetId:         task.RulesetId,
		DataHash:          docHash,
		LintStatus:        view.StatusSuccess,
		LintDetails:       "",
	}

	verEnt := entity.LintedVersion{
		PackageId:   task.PackageId,
		Version:     task.Version,
		Revision:    task.Revision,
		LintStatus:  view.VersionStatusInProgress,
		LintDetails: "",
		LintedAt:    time.Now(),
	}

	lintFileResult := &entity.LintFileResult{
		DataHash:      docHash,
		RulesetId:     task.RulesetId,
		LinterVersion: cached.LinterVersion,
		Data:          cached.Data,
		Summary:       cached.Summary,
	}

	err := d.docResultRepository.SaveLintResult(ctx, task.Id, view.StatusSuccess, "", lintTimeMs, verEnt, docEnt, lintFileResult, d.executorId)
	if err != nil {
		log.Errorf("Failed to save cached lint result for task %s: %s", task.Id, err)
	}
}

// TODO: temp! just for testing!
func (d docTaskProcessorImpl) writeAsyncTestLog(taskId string) {
	enabled := os.Getenv("TASK_LOG")
	if enabled == "" {
		return
	}
	fileName := "doc_task_log_" + d.executorId + ".txt"

	// Open the file in append mode, create it if it doesn't exist, with write-only permissions
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("failed to open test log entry file %s", fileName)
		return
	}
	defer file.Close()

	if _, err := file.WriteString(taskId + "\n"); err != nil {
		log.Errorf("failed to write test log entry to file %s", fileName)
		return
	}
}
