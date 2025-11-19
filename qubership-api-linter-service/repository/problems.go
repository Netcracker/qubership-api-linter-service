package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Netcracker/qubership-api-linter-service/db"
	"github.com/Netcracker/qubership-api-linter-service/entity"
)

type ProblemsRepository interface {
	SaveProblems(ctx context.Context, ent entity.Problems) error
	GetProblems(ctx context.Context, PackageId string, Version string, Revision int, OperationId string) (entity.Problems, error)
}

func NewProblemsRepository(cp db.ConnectionProvider) ProblemsRepository {
	return &problemsRepositoryImpl{cp: cp}
}

type problemsRepositoryImpl struct {
	cp db.ConnectionProvider
}

func (p problemsRepositoryImpl) SaveProblems(ctx context.Context, ent entity.Problems) error {
	_, err := p.cp.GetConnection().ModelContext(ctx, &ent).
		OnConflict("(package_id, version, revision, operation_id) DO UPDATE").
		Set("prompt_hash = EXCLUDED.prompt_hash").
		Set("problems = EXCLUDED.problems").
		Insert()
	return err
}

func (p problemsRepositoryImpl) GetProblems(ctx context.Context, PackageId string, Version string, Revision int, OperationId string) (entity.Problems, error) {
	var result entity.Problems
	err := p.cp.GetConnection().ModelContext(ctx, &result).
		Where("package_id = ?", PackageId).
		Where("version = ?", Version).
		Where("revision = ?", Revision).
		Where("operation_id = ?", OperationId).
		Select()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Problems{}, nil
		}
		return entity.Problems{}, err
	}
	return result, nil
}
