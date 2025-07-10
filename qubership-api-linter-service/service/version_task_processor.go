package service

import (
	"context"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/utils"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"time"
)

type VersionTaskProcessor interface {
	StartVersionLintTask(taskId string) error
}

func NewVersionTaskProcessor(verRepo repository.VersionLintTaskRepository, docRepo repository.DocLintTaskRepository, cl client.ApihubClient, linterSelectorService LinterSelectorService, executorId string) VersionTaskProcessor {
	svc := &versionTaskProcessorImpl{
		verRepo:               verRepo,
		docRepo:               docRepo,
		cl:                    cl,
		linterSelectorService: linterSelectorService,
		executorId:            executorId,
	}

	utils.SafeAsync(func() {
		svc.acquireFreeTasks()
	})

	return svc
}

type versionTaskProcessorImpl struct {
	verRepo               repository.VersionLintTaskRepository
	docRepo               repository.DocLintTaskRepository
	cl                    client.ApihubClient
	linterSelectorService LinterSelectorService
	executorId            string
}

func (v versionTaskProcessorImpl) StartVersionLintTask(taskId string) error {
	utils.SafeAsync(func() {
		v.processVersionLintTask(taskId)
	})
	return nil
}

func (v versionTaskProcessorImpl) processVersionLintTask(taskId string) {
	log.Debugf("Start processing version Lint task %s", taskId)

	ctx := context.Background()

	task, err := v.verRepo.GetTaskById(ctx, taskId)
	if err != nil {
		log.Errorf("Failed to get task by id %s: %s", taskId, err)
		return
	}
	if task.ExecutorId != v.executorId {
		log.Errorf("Version lint task id=%s executorId=%s does not match current executorId=%s", taskId, task.ExecutorId, v.executorId)
		return
	}

	err = v.verRepo.UpdateStatusAndDetails(ctx, taskId, view.StatusProcessing, "")
	if err != nil {
		log.Errorf("Failed to update task %s status to %s: %v", taskId, view.StatusProcessing, err)
		return
	}

	// TODO: update last_active for version task periodically in goroutine!!!!!!!!!

	version := fmt.Sprintf("%s@%d", task.Version, task.Revision)

	secC := secctx.CreateSystemContext()

	docs, err := v.cl.GetVersionDocuments(secC, task.PackageId, version)
	if err != nil {
		err = v.verRepo.UpdateStatusAndDetails(ctx, taskId, view.StatusError, fmt.Sprintf("Failed to get version documents: %s", err))
		if err != nil {
			log.Errorf("Failed to update task %s status to %s: %v", taskId, view.StatusError, err)
			return
		}
		return
	}

	type linterAndRuleset struct {
		linter    view.Linter
		rulesetId string
		err       error
	}

	typeToLinter := make(map[view.ApiType]linterAndRuleset)
	for _, doc := range docs.Documents {
		_, exists := typeToLinter[doc.Type]
		if !exists {
			linter, rulesetId, err := v.linterSelectorService.SelectLinterAndRuleset(doc.Type)

			typeToLinter[doc.Type] = linterAndRuleset{
				linter:    linter,
				rulesetId: rulesetId,
				err:       err,
			}
		}
	}

	var docTasks []entity.DocumentLintTask

	for _, doc := range docs.Documents {
		lr := typeToLinter[doc.Type]

		status := view.StatusNotStarted
		details := ""
		executorId := ""

		if lr.err != nil {
			status = view.StatusError
			details = lr.err.Error()
			executorId = v.executorId
		}

		if lr.rulesetId == "" {
			status = view.StatusError
			details = fmt.Sprintf("No suitable ruleset was found. Linter=%s", lr.linter)
			executorId = v.executorId
		}

		docTaskEnt := entity.DocumentLintTask{
			Id:                uuid.NewString(),
			VersionLintTaskId: taskId,
			PackageId:         task.PackageId,
			Version:           task.Version,
			Revision:          task.Revision,
			FileId:            doc.FieldId,
			FileSlug:          doc.Slug,
			APIType:           doc.Type,
			Linter:            lr.linter,
			RulesetId:         lr.rulesetId,
			Status:            status,
			Details:           details,
			CreatedAt:         time.Now(),
			ExecutorId:        executorId,
			RestartCount:      0,
			LastActive:        nil,
		}

		docTasks = append(docTasks, docTaskEnt)
	}

	err = v.docRepo.SaveDocTasksAndUpdVer(ctx, docTasks, taskId)
	if err != nil {
		err = v.verRepo.UpdateStatusAndDetails(ctx, taskId, view.StatusError, fmt.Sprintf("Failed to save doc tasks: %s", err))
		if err != nil {
			log.Errorf("Failed to update task %s status to %s: %v", taskId, view.StatusError, err)
			return
		}
		return
	}

	log.Infof("Version lint task with id %s is processed, %d document lint task(s) created", taskId, len(docTasks))
}

func (v versionTaskProcessorImpl) acquireFreeTasks() {

	// TODO: timer

	// get task from repo and pass to processVersionLintTask()

}
