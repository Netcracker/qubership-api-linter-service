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

	utils.SafeAsync(func() {
		svc.checkDocReady()
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
			LastActive:        nil,
			RestartCount:      0,
			Priority:          0,
			LintTimeMs:        0,
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
	t := time.NewTicker(time.Second * 5)
	ctx := context.Background()
	for range t.C {
		task, err := v.verRepo.FindFreeVersionTask(ctx, v.executorId)
		if err != nil {
			log.Errorf("Failed to find free version task: %s", err)
			continue
		}
		if task != nil {
			v.processVersionLintTask(task.Id)
		}
	}
}

func (v versionTaskProcessorImpl) checkDocReady() {
	t := time.NewTicker(time.Second * 5)
	ctx := context.Background()
	for range t.C {
		log.Infof("checkDocReady running") // TODO: remove

		verLintTasks, err := v.verRepo.GetWaitingForDocTasks(ctx, v.executorId)
		if err != nil {
			log.Errorf("Failed to get version tasks in waiting for docs status: %s", err)
			continue
		}
		if len(verLintTasks) == 0 {
			continue
		}
		var verTaskIds []string

		for _, task := range verLintTasks {
			verTaskIds = append(verTaskIds, task.Id)
		}

		docLintTasks, err := v.docRepo.GetDocTasksForVersionTasks(ctx, verTaskIds)
		if err != nil {
			log.Errorf("Failed to get doc lint tasks for readiness check: %s", err)
		}
		// don't expect many entries, so just iterating

		for _, verLintTask := range verLintTasks {
			var numSucceed int
			var numFailed int
			var numNotReady int
			for _, docLintTask := range docLintTasks {
				if docLintTask.VersionLintTaskId != verLintTask.Id {
					continue
				}
				switch docLintTask.Status {
				case view.StatusComplete:
					numSucceed++
					break
				case view.StatusError:
					numFailed++
					break
				case view.StatusNotStarted, view.StatusLinting, view.StatusProcessing:
					numNotReady++
					break
				default:
					log.Warnf("handleDocReady(): unexpected doc lint task status: %s", docLintTask.Status)
					break
				}
			}
			if numNotReady > 0 {
				// version task is not ready yet
				err = v.verRepo.UpdateStatusAndDetails(ctx, verLintTask.Id, view.StatusWaitingForDocs, "")
				if err != nil {
					log.Errorf("Failed to update version lint task %s status to %s: %v", verLintTask.Id, view.StatusWaitingForDocs, err)
					continue
				}
			} else {
				// version task is ready
				if numFailed > 0 {
					log.Infof("Version lint task %s is failed because of failed doc tasks", verLintTask.Id)
					err = v.verRepo.UpdateStatusAndDetails(ctx, verLintTask.Id, view.StatusError, fmt.Sprintf("%d doc lint task(s) failed", numFailed))
					if err != nil {
						log.Errorf("Failed to update version lint task %s status to %s: %v", verLintTask.Id, view.StatusError, err)
						continue
					}
				} else {
					log.Infof("Version lint task %s successfully completed", verLintTask.Id)
					err = v.verRepo.UpdateStatusAndDetails(ctx, verLintTask.Id, view.StatusComplete, "")
					if err != nil {
						log.Errorf("Failed to update version lint task %s status to %s: %v", verLintTask.Id, view.StatusComplete, err)
						continue
					}
				}
			}
		}

	}

}
