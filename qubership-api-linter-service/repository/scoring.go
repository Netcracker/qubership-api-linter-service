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
	GetScoresForDoc(ctx context.Context, PackageId string, Version string, Revision int, slug string) ([]entity.OperationScore, error)
}

func NewScoringRepository(cp db.ConnectionProvider) ScoringRepository {
	return &scoringRepositoryImpl{cp: cp}
}

type scoringRepositoryImpl struct {
	cp db.ConnectionProvider
}

func (s scoringRepositoryImpl) GetScoresForDoc(ctx context.Context, PackageId string, Version string, Revision int, slug string) ([]entity.OperationScore, error) {
	var results []entity.OperationScore
	err := s.cp.GetConnection().ModelContext(ctx, &results).
		TableExpr("scoring_operation AS score").
		ColumnExpr("score.*").
		Join("JOIN linted_operation lo ON lo.package_id = score.package_id AND lo.version = score.version AND lo.revision = score.revision AND lo.operation_id = score.operation_id").
		Where("score.package_id = ?", PackageId).
		Where("score.version = ?", Version).
		Where("score.revision = ?", Revision).
		Where("lo.slug = ?", slug).
		Order("score.operation_id ASC").
		Select()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []entity.OperationScore{}, nil
		}
		return nil, err
	}
	return results, nil
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
