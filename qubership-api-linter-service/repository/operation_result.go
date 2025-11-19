package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Netcracker/qubership-api-linter-service/db"
	"github.com/Netcracker/qubership-api-linter-service/entity"
)

type OperationResultRepository interface {
	GetOperationResult(ctx context.Context, dataHash string, rulesetId string) (*entity.LintOperationResult, error)
	GetOperationsForVersion(ctx context.Context, packageId, version string) ([]entity.LintedOperation, error)
}

func NewOperationResultRepository(cp db.ConnectionProvider) OperationResultRepository {
	return &operationResultRepositoryImpl{cp: cp}
}

type operationResultRepositoryImpl struct {
	cp db.ConnectionProvider
}

func (o operationResultRepositoryImpl) GetOperationsForVersion(ctx context.Context, packageId, version string) ([]entity.LintedOperation, error) {
	var operations []entity.LintedOperation
	err := o.cp.GetConnection().ModelContext(ctx, &operations).
		Where("package_id = ?", packageId).
		Where("version = ?", version).
		Select()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []entity.LintedOperation{}, nil
		}
		return nil, err
	}
	return operations, nil
}

func (o operationResultRepositoryImpl) GetOperationResult(ctx context.Context, dataHash string, rulesetId string) (*entity.LintOperationResult, error) {
	var result entity.LintOperationResult
	err := o.cp.GetConnection().ModelContext(ctx, &result).
		Where("data_hash = ?", dataHash).
		Where("ruleset_id = ?", rulesetId).
		Select()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}
