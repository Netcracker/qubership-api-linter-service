// Copyright 2024-2025 NetCracker Technology Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/utils"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type ValidationService interface {
	ValidateVersion(ctx context.Context, packageId string, version string, eventId string) (string, error)
	GetVersionSummary(ctx context.Context, packageId string, version string) (*view.ValidationSummaryForVersion, error)
	GetValidationResult(ctx context.Context, packageId string, version string, slug string) (*view.DocumentResult, error)
	StartBulkValidation(ctx context.Context, req view.BulkValidationRequest) (string, error)
	GetBulkValidationStatus(ctx context.Context, jobId string) (*view.BulkValidationStatusResponse, error)
}

func NewValidationService(
	verTaskRepo repository.VersionLintTaskRepository,
	versionResultRepository repository.VersionResultRepository,
	lintResultRepository repository.LintResultRepository,
	rulesetRepository repository.RulesetRepository,
	docLintTaskRepository repository.DocLintTaskRepository,
	versionTaskProcessor VersionTaskProcessor,
	apihubClient client.ApihubClient,
	executorId string) ValidationService {
	return &validationServiceImpl{
		verTaskRepo:             verTaskRepo,
		versionResultRepository: versionResultRepository,
		lintResultRepository:    lintResultRepository,
		rulesetRepository:       rulesetRepository,
		docLintTaskRepository:   docLintTaskRepository,
		versionTaskProcessor:    versionTaskProcessor,
		apihubClient:            apihubClient,
		executorId:              executorId,
		bulkJobs:                make(map[string]*bulkValidationJob),
	}
}

type validationServiceImpl struct {
	verTaskRepo             repository.VersionLintTaskRepository
	versionResultRepository repository.VersionResultRepository
	lintResultRepository    repository.LintResultRepository
	rulesetRepository       repository.RulesetRepository
	docLintTaskRepository   repository.DocLintTaskRepository

	versionTaskProcessor VersionTaskProcessor
	apihubClient         client.ApihubClient
	executorId           string

	bulkJobs      map[string]*bulkValidationJob
	bulkJobsMutex sync.RWMutex
}

type bulkValidationJob struct {
	mu                sync.Mutex
	status            view.AsyncStatus
	totalVersions     int
	processedVersions int
	errorMessage      string
	entries           []view.BulkValidationEntry
}

func (v *validationServiceImpl) GetVersionSummary(ctx context.Context, packageId string, version string) (*view.ValidationSummaryForVersion, error) {
	ver, rev, err := getVersionAndRevision(ctx, v.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	lintedVer, lintedDocs, err := v.versionResultRepository.GetVersionAndDocsSummary(ctx, packageId, ver, rev)
	if err != nil {
		return nil, err
	}
	if lintedVer == nil {
		// version is not linted (yet), need to check if lint is planned/in progress
		varTasks, err := v.verTaskRepo.GetRunningTaskForVersion(ctx, packageId, ver, rev)
		if err != nil {
			return nil, err
		}
		if len(varTasks) == 0 {
			return nil, nil
		}
		return &view.ValidationSummaryForVersion{
			Status:    view.VersionStatusInProgress,
			Details:   "",
			Documents: nil,
			Rulesets:  nil,
		}, nil
	}

	result := &view.ValidationSummaryForVersion{
		Status:    lintedVer.LintStatus,
		Details:   lintedVer.LintDetails,
		Documents: nil,
		Rulesets:  nil,
	}

	rulesetMap, err := v.makeRulesetMap(ctx, makeRulesetIdsFromLintedDocs(lintedDocs))
	if err != nil {
		return nil, err
	}

	for _, doc := range lintedDocs {
		if doc.LintStatus == view.StatusError {
			result.Documents = append(result.Documents, view.ValidationDocument{
				Status:       doc.LintStatus,
				Details:      doc.LintDetails,
				Slug:         doc.Slug,
				ApiType:      doc.SpecificationType,
				DocumentName: doc.FileId,
				RulesetId:    doc.RulesetId,
			})
			continue
		}
		resultSummary, err := v.lintResultRepository.GetLintResultSummary(ctx, doc.DataHash, doc.RulesetId)
		if err != nil {
			return nil, err
		}
		if resultSummary == nil {
			continue
		}

		ruleset, ok := rulesetMap[doc.RulesetId]
		if !ok {
			return nil, fmt.Errorf("ruleset with id %s is not found in cache map", doc.RulesetId)
		}

		var summ *view.IssuesSummary

		switch ruleset.Linter {
		case view.SpectralLinter:
			// calculate spectral summary
			summ, err = makeSpectralSummary(resultSummary.Summary)
			if err != nil {
				return nil, err
			}
			if summ == nil {
				return nil, fmt.Errorf("failed to calculate spectral result summary")
			}
		case view.UnknownLinter:
			return nil, fmt.Errorf("unknown linter %s", ruleset.Linter)
		default:
			return nil, fmt.Errorf("unknown linter %s", ruleset.Linter)
		}

		result.Documents = append(result.Documents, view.ValidationDocument{
			Status:       doc.LintStatus,
			Details:      doc.LintDetails,
			Slug:         doc.Slug,
			ApiType:      doc.SpecificationType,
			DocumentName: doc.FileId,
			RulesetId:    doc.RulesetId,
			IssuesSummary: &view.IssuesSummary{
				Error:   summ.Error,
				Warning: summ.Warning,
				Info:    summ.Info,
				Hint:    summ.Hint,
			},
		})
	}

	for _, val := range rulesetMap {
		result.Rulesets = append(result.Rulesets, entity.MakeRulesetView(val))
	}
	return result, nil
}

func (v *validationServiceImpl) GetValidationResult(ctx context.Context, packageId string, version string, slug string) (*view.DocumentResult, error) {
	ver, rev, err := getVersionAndRevision(ctx, v.apihubClient, packageId, version)
	if err != nil {
		return nil, err
	}

	lintedDocument, err := v.versionResultRepository.GetLintedDocument(ctx, packageId, ver, rev, slug)
	if err != nil {
		return nil, err
	}
	if lintedDocument == nil {
		return nil, nil
	}

	ruleset, err := v.rulesetRepository.GetRulesetById(ctx, lintedDocument.RulesetId)
	if err != nil {
		return nil, err
	}
	if ruleset == nil {
		return nil, fmt.Errorf("ruleset with id %s not found", lintedDocument.RulesetId)
	}

	if lintedDocument.LintStatus == view.StatusError {
		result := view.DocumentResult{
			Ruleset:           entity.MakeRulesetView(*ruleset),
			Issues:            nil,
			ValidatedDocument: entity.MakeValidatedDocumentView(*lintedDocument),
		}
		return &result, nil
	}

	lintResult, err := v.lintResultRepository.GetLintResult(ctx, lintedDocument.DataHash, lintedDocument.RulesetId)
	if err != nil {
		return nil, err
	}
	if lintResult == nil {
		return nil, nil
	}

	issues := make([]view.ValidationIssue, 0)
	var spectralOutput []view.SpectralOutputItem
	err = json.Unmarshal(lintResult.Data, &spectralOutput)
	if err != nil {
		return nil, err
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

	result := view.DocumentResult{
		Ruleset:           entity.MakeRulesetView(*ruleset),
		Issues:            issues,
		ValidatedDocument: entity.MakeValidatedDocumentView(*lintedDocument),
	}

	return &result, nil
}

func makeSpectralSummary(summary map[string]interface{}) (*view.IssuesSummary, error) {
	result := view.IssuesSummary{}
	errCStr, ok := summary["errorCount"] // summary could be empty
	if ok {
		if errC, ok := errCStr.(float64); ok {
			result.Error = int(errC)
		}
	}
	warningCStr, ok := summary["warningCount"]
	if ok {
		if warningC, ok := warningCStr.(float64); ok {
			result.Warning = int(warningC)
		}
	}

	infoCStr, ok := summary["infoCount"]
	if ok {
		if infoC, ok := infoCStr.(float64); ok {
			result.Info = int(infoC)
		}
	}

	hintCStr, ok := summary["hintCount"]
	if ok {
		if infoC, ok := hintCStr.(float64); ok {
			result.Hint = int(infoC)
		}
	}

	return &result, nil
}

func (v *validationServiceImpl) ValidateVersion(ctx context.Context, packageId string, version string, eventId string) (string, error) {
	pkg, err := v.apihubClient.GetPackageById(ctx, packageId)
	if err != nil {
		return "", err
	}
	if pkg.Kind != string(view.KindPackage) {
		return "", &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.LintNotSupported,
			Message: exception.LintNotSupportedMsg,
			Params: map[string]interface{}{
				"$kind": pkg.Kind,
				"$id":   pkg.Id,
			},
		}
	}

	ver, rev, err := getVersionAndRevision(ctx, v.apihubClient, packageId, version)
	if err != nil {
		return "", err
	}

	userId := secctx.GetUserId(ctx)

	ent := entity.VersionLintTask{
		Id:           uuid.NewString(),
		PackageId:    packageId,
		Version:      ver,
		Revision:     rev,
		Status:       view.TaskStatusNotStarted,
		Details:      "",
		CreatedAt:    time.Now(),
		CreatedBy:    userId,
		LastActive:   time.Now(),
		EventId:      eventId, // optional
		RestartCount: 0,
		Priority:     0,
	}
	err = v.verTaskRepo.SaveVersionTask(context.Background(), ent)
	if err != nil {
		return "", err
	}

	return ent.Id, nil
}

func (v *validationServiceImpl) StartBulkValidation(ctx context.Context, req view.BulkValidationRequest) (string, error) {
	if req.PackageId == "" {
		return "", &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.RequiredParamsMissing,
			Message: exception.RequiredParamsMissingMsg,
			Params:  map[string]interface{}{"params": "packageId"},
		}
	}

	rootPackage, err := v.apihubClient.GetPackageById(ctx, req.PackageId)
	if err != nil {
		return "", err
	}
	if rootPackage == nil {
		return "", &exception.CustomError{
			Status:  http.StatusNotFound,
			Code:    exception.EntityNotFound,
			Message: exception.EntityNotFoundMsg,
			Params: map[string]interface{}{
				"entity": "package",
				"id":     req.PackageId,
			},
		}
	}

	excludeSet := make(map[string]struct{}, len(req.ExcludePackages))
	for _, id := range req.ExcludePackages {
		excludeSet[id] = struct{}{}
	}

	targetPackages, err := v.getTargetPackages(ctx, *rootPackage, excludeSet)
	if err != nil {
		return "", err
	}

	jobId := uuid.NewString()
	job := &bulkValidationJob{
		status:            view.ESProcessing,
		entries:           make([]view.BulkValidationEntry, 0),
		totalVersions:     0,
		processedVersions: 0,
	}

	if len(targetPackages) == 0 {
		job.status = view.ESSuccess
		v.bulkJobsMutex.Lock()
		v.bulkJobs[jobId] = job
		v.bulkJobsMutex.Unlock()
		return jobId, nil
	}

	v.bulkJobsMutex.Lock()
	v.bulkJobs[jobId] = job
	v.bulkJobsMutex.Unlock()

	asyncCtx := secctx.MakeSysadminContext(context.Background())
	utils.SafeAsync(func() {
		v.runBulkValidationJob(asyncCtx, jobId, targetPackages, req.Version)
	})
	log.Infof("Bulk validation started for root package %s, jobId is: %s", req.PackageId, jobId)

	return jobId, nil
}

func (v *validationServiceImpl) GetBulkValidationStatus(ctx context.Context, jobId string) (*view.BulkValidationStatusResponse, error) {
	v.bulkJobsMutex.RLock()
	job, exists := v.bulkJobs[jobId]
	v.bulkJobsMutex.RUnlock()
	if !exists {
		return nil, &exception.CustomError{
			Status:  http.StatusNotFound,
			Code:    exception.EntityNotFound,
			Message: exception.EntityNotFoundMsg,
			Params: map[string]interface{}{
				"entity": "bulk validation job",
				"id":     jobId,
			},
		}
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	packages := make([]view.BulkValidationEntry, len(job.entries))
	copy(packages, job.entries)

	return &view.BulkValidationStatusResponse{
		JobId:             jobId,
		Status:            job.status,
		ProcessedVersions: job.processedVersions,
		TotalVersions:     job.totalVersions,
		Error:             job.errorMessage,
		Packages:          packages,
	}, nil
}

func (v *validationServiceImpl) runBulkValidationJob(ctx context.Context, jobId string, packageIds []string, versionFilter string) {
	job := v.getBulkJob(jobId)
	if job == nil {
		return
	}

	job.mu.Lock()
	job.status = view.ESProcessing
	job.mu.Unlock()

	type scheduleItem struct {
		packageId string
		version   string
	}

	var schedule []scheduleItem
	for _, pkgId := range packageIds {
		versions, err := v.collectPackageVersions(ctx, pkgId, versionFilter)
		if err != nil {
			job.mu.Lock()
			job.status = view.ESError
			job.errorMessage = err.Error()
			job.mu.Unlock()
			return
		}
		for _, ver := range versions {
			if ver == "" {
				continue
			}
			schedule = append(schedule, scheduleItem{packageId: pkgId, version: ver})
		}
	}

	job.mu.Lock()
	job.totalVersions = len(schedule)
	job.entries = make([]view.BulkValidationEntry, 0, len(schedule))
	job.mu.Unlock()

	if len(schedule) == 0 {
		job.mu.Lock()
		job.status = view.ESSuccess
		job.mu.Unlock()
		return
	}

	for _, item := range schedule {
		taskId, err := v.ValidateVersion(ctx, item.packageId, item.version, "")

		job.mu.Lock()
		entry := view.BulkValidationEntry{
			PackageId:         item.packageId,
			Version:           item.version,
			ValidationStarted: err == nil,
		}
		if err == nil {
			entry.ValidationTaskId = taskId
			job.processedVersions++
		} else {
			job.status = view.ESError
			job.errorMessage = err.Error()
		}
		job.entries = append(job.entries, entry)
		job.mu.Unlock()

		if err != nil {
			return
		}
	}

	job.mu.Lock()
	job.status = view.ESSuccess
	job.mu.Unlock()
}

func (v *validationServiceImpl) getTargetPackages(ctx context.Context, root view.SimplePackage, exclude map[string]struct{}) ([]string, error) {
	if _, skip := exclude[root.Id]; skip {
		return []string{}, nil
	}

	switch root.Kind {
	case string(view.KindPackage):
		return []string{root.Id}, nil
	case string(view.KindGroup):
		var result []string

		limit := 100
		page := 0
		for {
			packages, err := v.apihubClient.GetPackagesList(ctx, view.PackageListReq{
				ParentID: root.Id,
				Kind:     []string{string(view.KindPackage)},
				Page:     &page,
				Limit:    &limit,
			})
			if err != nil {
				return nil, err
			}

			if packages == nil {
				return []string{}, nil
			}

			for _, pkg := range packages.Packages {
				if _, skip := exclude[pkg.Id]; skip {
					continue
				}
				result = append(result, pkg.Id)
			}

			if len(packages.Packages) < limit {
				break
			}
			page++
		}

		return result, nil
	default:
		return nil, &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.LintNotSupported,
			Message: exception.LintNotSupportedMsg,
			Params: map[string]interface{}{
				"$kind": root.Kind,
				"$id":   root.Id,
			},
		}
	}
}

func (v *validationServiceImpl) collectPackageVersions(ctx context.Context, packageId string, versionFilter string) ([]string, error) {
	if versionFilter != "" {
		return []string{versionFilter}, nil
	}

	versions, err := v.apihubClient.ListPackageVersions(ctx, packageId)
	if err != nil {
		return nil, err
	}
	if versions == nil {
		return []string{}, nil
	}

	result := make([]string, 0, len(versions))
	for _, ver := range versions {
		if ver.Version != "" {
			result = append(result, ver.Version)
		}
	}
	return result, nil
}

func (v *validationServiceImpl) getBulkJob(jobId string) *bulkValidationJob {
	v.bulkJobsMutex.RLock()
	job, exists := v.bulkJobs[jobId]
	v.bulkJobsMutex.RUnlock()
	if !exists {
		return nil
	}
	return job
}

const tempFolder = "tmp"

func (v *validationServiceImpl) makeRulesetMap(ctx context.Context, rulesetIds []string) (map[string]entity.Ruleset, error) {
	rulesetMap := make(map[string]entity.Ruleset)
	for _, rulesetId := range rulesetIds {
		_, exists := rulesetMap[rulesetId]
		if !exists {
			ruleset, err := v.rulesetRepository.GetRulesetById(ctx, rulesetId)
			if err != nil {
				return nil, err
			}
			if ruleset == nil {
				return nil, fmt.Errorf("ruleset with id %s not found", rulesetId)
			}
			rulesetMap[rulesetId] = *ruleset
		}
	}
	return rulesetMap, nil
}

func makeRulesetIdsFromTasks(tasks []entity.DocumentLintTask) []string {
	var result []string
	for _, task := range tasks {
		result = append(result, task.RulesetId)
	}
	return result
}

func makeRulesetIdsFromLintedDocs(tasks []entity.LintedDocument) []string {
	var result []string
	for _, task := range tasks {
		result = append(result, task.RulesetId)
	}
	return result
}
