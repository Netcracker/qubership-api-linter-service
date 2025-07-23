package repository

import (
	"context"
	"github.com/Netcracker/qubership-api-linter-service/db"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/go-pg/pg/v10"
)

type DocResultRepository interface {
	LintResultExists(ctx context.Context, dataHash string) (bool, error)
	SaveLintResult(ctx context.Context, docLintTaskId string, lintTimeMs int64, version entity.LintedVersion, document entity.LintedDocument, result *entity.LintFileResult) error
}

func NewDocResultRepository(cp db.ConnectionProvider) DocResultRepository {
	return &docResultRepositoryImpl{cp: cp}
}

type docResultRepositoryImpl struct {
	cp db.ConnectionProvider
}

func (d docResultRepositoryImpl) LintResultExists(ctx context.Context, dataHash string) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (d docResultRepositoryImpl) SaveLintResult(ctx context.Context, docLintTaskId string, lintTimeMs int64, version entity.LintedVersion, document entity.LintedDocument, result *entity.LintFileResult) error {
	return d.cp.GetConnection().RunInTransaction(ctx, func(tx *pg.Tx) error {
		_, err := tx.Model(&version).OnConflict("(package_id, version, revision) do update").Insert()
		if err != nil {
			return err
		}
		_, err = tx.Model(&document).OnConflict("(package_id, version, revision, file_id) do update").Insert()
		if err != nil {
			return err
		}
		if result != nil {
			_, err = tx.Model(result).OnConflict("(data_hash, ruleset_id) do update").Insert()
			if err != nil {
				return err
			}
		}
		var docLintTask *entity.DocumentLintTask
		_, err = tx.Model(docLintTask).
			Set("status = ?", view.TaskStatusComplete).
			Set("last_active = now()").
			Set("lint_time_ms = ?", lintTimeMs).
			Where("id = ?", docLintTaskId).
			Update()
		if err != nil {
			return err
		}
		return nil
	})
}
