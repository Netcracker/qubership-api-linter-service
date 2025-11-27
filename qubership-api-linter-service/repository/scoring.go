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

/*
	err = d.cp.GetConnection().Model(changedVersion).
		ColumnExpr(`migrated_version_changes.*,
				b.metadata->>'build_type' build_type,
				b.metadata->>'previous_version' previous_version,
				b.metadata->>'previous_version_package_id' previous_version_package_id`).
		Join("inner join build b").
		JoinOn("migrated_version_changes.build_id = b.build_id").
		Where("migrated_version_changes.migration_id = ?", migrationId).
		Where("? = any(unique_changes)", change).
		Order("build_id").
		Limit(1).
		Select()
*/

func (s scoringRepositoryImpl) GetScoresForDoc(ctx context.Context, PackageId string, Version string, Revision int, slug string) ([]entity.OperationScore, error) {
	var results []entity.OperationScore

	err := s.cp.GetConnection().ModelContext(ctx, &results).
		ColumnExpr("scoring_operation.*").
		Join("inner join linted_operation lo ").
		JoinOn("lo.package_id = scoring_operation.package_id AND lo.version = scoring_operation.version AND lo.revision = scoring_operation.revision AND lo.operation_id = scoring_operation.operation_id").
		Where("scoring_operation.package_id = ?", PackageId).
		Where("scoring_operation.version = ?", Version).
		Where("scoring_operation.revision = ?", Revision).
		Where("lo.slug = ?", slug).
		Order("scoring_operation.operation_id ASC").
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
