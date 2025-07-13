package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/db"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/go-pg/pg/v10"
	"net/http"
	"strings"
	"time"
)

type VersionLintTaskRepository interface {
	SaveVersionTask(ctx context.Context, ent entity.VersionLintTask) error
	GetTaskById(ctx context.Context, taskId string) (*entity.VersionLintTask, error)
	UpdateStatusAndDetails(ctx context.Context, taskId string, status view.TaskStatus, details string) error
	FindFreeVersionTask(ctx context.Context, executorId string) (*entity.VersionLintTask, error)
	GetWaitingForDocTasks(ctx context.Context, executorId string) ([]entity.VersionLintTask, error)
}

type versionLintTaskRepositoryImpl struct {
	cp db.ConnectionProvider
}

func (r *versionLintTaskRepositoryImpl) GetWaitingForDocTasks(ctx context.Context, executorId string) ([]entity.VersionLintTask, error) {
	var result []entity.VersionLintTask
	err := r.cp.GetConnection().ModelContext(ctx, &result).
		Where("status = ?", view.StatusWaitingForDocs).
		Where("executor_id = ?", executorId).
		Select()
	if err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

func NewVersionLintTaskRepository(cp db.ConnectionProvider) VersionLintTaskRepository {
	return &versionLintTaskRepositoryImpl{cp: cp}
}

func (r *versionLintTaskRepositoryImpl) SaveVersionTask(ctx context.Context, ent entity.VersionLintTask) error {
	_, err := r.cp.GetConnection().ModelContext(ctx, &ent).Insert()
	if err != nil {
		var pgerr pg.Error
		if errors.As(err, &pgerr) {
			if pgerr.Field('C') == "23505" && strings.Contains(err.Error(), "version_lint_task_event_id_unique") { // ERROR #23505 duplicate key value violates unique constraint "version_lint_task_event_id_unique"
				return &exception.CustomError{
					Status:  http.StatusInternalServerError,
					Code:    exception.DuplicateEvent,
					Message: exception.DuplicateEventMsg,
					Params:  map[string]interface{}{"event_id": ent.EventId},
				}
			}
		}
		return err
	}
	return nil
}

func (r *versionLintTaskRepositoryImpl) UpdateStatusAndDetails(ctx context.Context, taskId string, status view.TaskStatus, details string) error {
	var ent entity.VersionLintTask
	_, err := r.cp.GetConnection().ModelContext(ctx, &ent).
		Set("status = ?", status).
		Set("details = ?", details).
		Set("last_active = ?", time.Now()).
		Where("id = ?", taskId).
		Update()
	return err
}

func (r *versionLintTaskRepositoryImpl) GetTaskById(ctx context.Context, taskId string) (*entity.VersionLintTask, error) {
	var task entity.VersionLintTask
	err := r.cp.GetConnection().ModelContext(ctx, &task).
		Where("id = ?", taskId).
		Select()
	if err != nil {
		return nil, err
	}
	return &task, nil
}

/*func (r *versionLintTaskRepositoryImpl) FindFreeVersionTask(ctx context.Context, executorId string) (*entity.VersionLintTask, error) {
	var task entity.VersionLintTask

	err := r.cp.GetConnection().RunInTransaction(ctx, func(tx *pg.Tx) error {
		err := tx.ModelContext(ctx, &task).
			Where("status = ?", "none").  // TODO: constant here
			Where("executor_id IS NULL"). // TODO: or restarts, last_active
			Order("created_at ASC").
			For("UPDATE SKIP LOCKED").
			Limit(1).
			Select()
		if err != nil {
			return err
		}

		// Update the task to assign it
		task.ExecutorId = executorId
		task.Status = "running"
		// todo: last active
		_, err = tx.ModelContext(ctx, &task).
			WherePK().
			Update()
		return err
	})
	if err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &task, err
}*/

var queryVersionTask = fmt.Sprintf("select * from version_lint_task b where "+
	"(b.status='%s' or ((b.status='%s' or b.status='%s') and b.last_active < (now() - interval '%d seconds'))) "+
	"order by b.created_at ASC limit 1 for no key update skip locked", view.StatusNotStarted, view.StatusProcessing, view.StatusWaitingForDocs, buildKeepaliveTimeoutSec)

func (r *versionLintTaskRepositoryImpl) FindFreeVersionTask(ctx context.Context, executorId string) (*entity.VersionLintTask, error) {
	var result *entity.VersionLintTask
	var err error

	for {
		taskFailed := false
		taskWaiting := false
		err = r.cp.GetConnection().RunInTransaction(context.Background(), func(tx *pg.Tx) error {
			var ents []entity.VersionLintTask

			_, err := tx.Query(&ents, queryVersionTask)
			if err != nil {
				if err == pg.ErrNoRows {
					return nil
				}
				return fmt.Errorf("failed to find free build: %w", err)
			}
			if len(ents) > 0 {
				result = &ents[0]

				if result.Status == view.StatusWaitingForDocs {
					// just update executor id
					_, err = tx.Model(result).
						Set("executor_id = ?", executorId).
						Set("last_active = ?", time.Now()).
						Where("id = ?", result.Id).
						Update()
					if err != nil {
						return err
					}
					taskWaiting = true
					result = nil
					return nil
				}

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
					result = nil
					return nil
				}

				// take free task
				isFirstRun := result.Status == view.StatusNotStarted

				if !isFirstRun {
					result.RestartCount += 1
				}

				result.Status = view.StatusProcessing
				result.ExecutorId = executorId

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
		if taskFailed || taskWaiting {
			continue
		}
		break
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}
