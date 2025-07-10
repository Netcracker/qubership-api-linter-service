package repository

import (
	"context"
	"errors"
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
	FindFreeVersionTask(ctx context.Context, executorId string) (entity.VersionLintTask, error)
}

type versionLintTaskRepositoryImpl struct {
	cp db.ConnectionProvider
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

func (r *versionLintTaskRepositoryImpl) FindFreeVersionTask(ctx context.Context, executorId string) (entity.VersionLintTask, error) {
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

	if errors.Is(err, pg.ErrNoRows) {
		return entity.VersionLintTask{}, pg.ErrNoRows
	}
	return task, err
}
