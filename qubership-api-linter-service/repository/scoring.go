package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Netcracker/qubership-api-linter-service/db"
	"github.com/Netcracker/qubership-api-linter-service/entity"
)

type ScoringRepository interface {
	SaveScore(ctx context.Context, ent entity.OperationScore) error
	GetScore(ctx context.Context, PackageId string, Version string, Revision int, OperationId string) (entity.OperationScore, error)
}

func NewScoringRepository(cp db.ConnectionProvider) ScoringRepository {
	return &scoringRepositoryImpl{cp: cp}
}

type scoringRepositoryImpl struct {
	cp db.ConnectionProvider
}

func (s scoringRepositoryImpl) SaveScore(ctx context.Context, ent entity.OperationScore) error {
	_, err := s.cp.GetConnection().ModelContext(ctx, &ent).
		OnConflict("(package_id, version, revision, operation_id) DO UPDATE").
		Set("score = EXCLUDED.score").
		Insert()
	return err
}

func (s scoringRepositoryImpl) GetScore(ctx context.Context, PackageId string, Version string, Revision int, OperationId string) (entity.OperationScore, error) {
	var result entity.OperationScore
	err := s.cp.GetConnection().ModelContext(ctx, &result).
		Where("package_id = ?", PackageId).
		Where("version = ?", Version).
		Where("revision = ?", Revision).
		Where("operation_id = ?", OperationId).
		Select()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.OperationScore{}, nil
		}
		return entity.OperationScore{}, err
	}
	return result, nil
}
