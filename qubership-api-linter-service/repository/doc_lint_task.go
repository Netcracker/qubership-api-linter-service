package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/db"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/go-pg/pg/v10"
	"time"
)

type DocLintTaskRepository interface {
	SetDocTaskStatus(ctx context.Context, docTaskId string, status view.TaskStatus, details string) error
	SaveDocTasksAndUpdVer(ctx context.Context, ents []entity.DocumentLintTask, versionTaskId string) error
	FindFreeDocTask(ctx context.Context, executorId string) (*entity.DocumentLintTask, error)
	//CheckDocTasksFinished(ctx context.Context, verTaskIds []string) ([]string, error)
	GetDocTasksForVersionTasks(ctx context.Context, verTaskIds []string) ([]entity.DocumentLintTask, error)
}

func NewDocLintTaskRepository(cp db.ConnectionProvider) DocLintTaskRepository {
	return &docLintTaskRepositoryImpl{cp: cp}
}

type docLintTaskRepositoryImpl struct {
	cp db.ConnectionProvider
}

func (d docLintTaskRepositoryImpl) SetDocTaskStatus(ctx context.Context, docTaskId string, status view.TaskStatus, details string) error {
	var verEnt entity.VersionLintTask

	_, err := d.cp.GetConnection().WithContext(ctx).Model(&verEnt).
		Set("status=?", status).
		Set("details=?", details).
		Set("last_active=?", time.Now()).
		Where("id=?", docTaskId).Update()
	if err != nil {
		return err
	}
	return nil
}

func (d docLintTaskRepositoryImpl) SaveDocTasksAndUpdVer(ctx context.Context, ents []entity.DocumentLintTask, versionTaskId string) error {
	err := d.cp.GetConnection().RunInTransaction(ctx, func(tx *pg.Tx) error {
		_, err := tx.Model(&ents).Insert()
		if err != nil {
			return err
		}

		allFailed := true
		for _, ent := range ents {
			if ent.Status != view.StatusError {
				allFailed = false
				break
			}
		}

		verStatus := view.StatusWaitingForDocs
		if allFailed {
			verStatus = view.StatusError
		}

		var verEnt entity.VersionLintTask
		_, err = tx.Model(&verEnt).
			Set("status=?", verStatus).
			Set("last_active=?", time.Now()).
			Where("id=?", versionTaskId).Update()
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

/*var queryItemToBuild = fmt.Sprintf("select * from build b where "+
"(b.status='none' or (b.status='%s' and b.last_active < (now() - interval '%d seconds'))) and "+
"(b.build_id not in (select distinct build_id from build_depends where depend_id in (select build.build_id from build where status='%s' or status='%s'))) "+
"order by b.priority DESC, b.created_at ASC limit 1 for no key update skip locked", view.StatusRunning, buildKeepaliveTimeoutSec, view.StatusNotStarted, view.StatusRunning)
*/

const buildKeepaliveTimeoutSec = 30

var queryItemToBuild = fmt.Sprintf("select * from document_lint_task b where "+
	"(b.status='%s' or (b.status='%s' and b.last_active < (now() - interval '%d seconds'))) "+
	"order by b.created_at ASC limit 1 for no key update skip locked", view.StatusNotStarted, view.StatusProcessing, buildKeepaliveTimeoutSec)

func (d docLintTaskRepositoryImpl) FindFreeDocTask(ctx context.Context, executorId string) (*entity.DocumentLintTask, error) {
	var result *entity.DocumentLintTask
	var err error

	for {
		taskFailed := false
		err = d.cp.GetConnection().RunInTransaction(context.Background(), func(tx *pg.Tx) error {
			var ents []entity.DocumentLintTask

			_, err := tx.Query(&ents, queryItemToBuild)
			if err != nil {
				if err == pg.ErrNoRows {
					return nil
				}
				return fmt.Errorf("failed to find free build: %w", err)
			}
			if len(ents) > 0 {
				result = &ents[0]

				// we got build candidate
				if result.RestartCount >= 2 {
					query := tx.Model(result).
						Where("id = ?", result.Id).
						Set("status = ?", view.StatusError).
						Set("details = ?", fmt.Sprintf("Restart count exceeded limit. Details: %v", result.Details)).
						Set("last_active = now()")
					_, err := query.Update()
					if err != nil {
						return err
					}
					taskFailed = true
					return nil
				}

				// take free task
				isFirstRun := result.Status == view.StatusNotStarted

				if !isFirstRun {
					result.RestartCount += 1
				}

				result.Status = view.StatusProcessing
				result.ExecutorId = executorId
				// TODO: add optimistic lock as well?

				_, err = tx.Model(result).
					Set("status = ?status").
					Set("executor_id = ?executor_id").
					Set("restart_count = ?restart_count").
					Set("last_active = now()").
					Where("id = ?", result.Id).
					Update()
				if err != nil {
					return fmt.Errorf("unable to update doc task status during takeTask: %w", err)
				}

				return nil
			}
			return nil
		})
		if taskFailed {
			continue
		}
		break
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (d docLintTaskRepositoryImpl) GetDocTasksForVersionTasks(ctx context.Context, verTaskIds []string) ([]entity.DocumentLintTask, error) {
	var result []entity.DocumentLintTask

	err := d.cp.GetConnection().WithContext(ctx).Model(&result).Where("version_lint_task_id in (?)", pg.In(verTaskIds)).Select()
	if err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}
