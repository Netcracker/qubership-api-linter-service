package repository

import (
	"context"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/db"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/go-pg/pg/v10"
	"time"
)

type DocLintTaskRepository interface {
	SaveDocTasksAndUpdVer(ctx context.Context, ents []entity.DocumentLintTask, versionTaskId string) error
	FindFreeDocTask(ctx context.Context, executorId string) (*entity.DocumentLintTask, error)
}

func NewDocLintTaskRepository(cp db.ConnectionProvider) DocLintTaskRepository {
	return &docLintTaskRepositoryImpl{cp: cp}
}

type docLintTaskRepositoryImpl struct {
	cp db.ConnectionProvider
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

/*
func (b buildRepositoryImpl) FindAndTakeFreeBuild(builderId string) (*entity.BuildEntity, error) {
	var result *entity.BuildEntity
	var err error
	for {
		buildFailed := false
		err = b.cp.GetConnection().RunInTransaction(context.Background(), func(tx *pg.Tx) error {
			var ents []entity.BuildEntity

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
						Where("build_id = ?", result.BuildId).
						Set("status = ?", view.StatusError).
						Set("details = ?", fmt.Sprintf("Restart count exceeded limit. Details: %v", result.Details)).
						Set("last_active = now()")
					_, err := query.Update()
					if err != nil {
						return err
					}
					buildFailed = true
					return nil
				}

				// take free build
				isFirstRun := result.Status == string(view.StatusNotStarted)

				if !isFirstRun {
					result.RestartCount += 1
				}

				result.Status = string(view.StatusRunning)
				result.BuilderId = builderId
				// TODO: add optimistic lock as well?

				_, err = tx.Model(result).
					Set("status = ?status").
					Set("builder_id = ?builder_id").
					Set("restart_count = ?restart_count").
					Set("last_active = now()").
					Where("build_id = ?", result.BuildId).
					Update()
				if err != nil {
					return fmt.Errorf("unable to update build status during takeBuild: %w", err)
				}

				return nil
			}
			return nil
		})
		if buildFailed {
			continue
		}
		break
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}
*/

/*
create table document_lint_task
(
    id                   varchar
        constraint doc_tasks_pk primary key,
    version_lint_task_id varchar                     not null,

    package_id           varchar                     not null, -- or join version_lint_task?
    version              varchar                     not null, -- or join version_lint_task?
    revision             integer                     not null, -- or join version_lint_task?
    file_id              varchar                     not null,

    api_type             varchar                     not null,
    linter               varchar                     not null,
    ruleset_id           varchar
        constraint ruleset_document_lint_task_ruleset_id_fk
            references ruleset (id),
    created_at           timestamp without time zone not null,

    status               varchar                     not null, -- task status
    details              varchar,
    executor_id          varchar,
    last_active          timestamp without time zone,
    restart_count        integer,
    lint_time_ms         integer
);
*/

// TODO: need to update version task status to doc_running
