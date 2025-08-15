package service

import (
	"context"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/google/uuid"
	"net/http"
	"time"
)

type RulesetService interface {
	CreateRuleset(ctx context.Context, name string, apiType view.ApiType, linter string, filename string, data []byte) (*view.Ruleset, error)
	ActivateRuleset(ctx context.Context, id string) error
	ListRulesets(ctx context.Context) ([]view.Ruleset, error)
	GetRuleset(ctx context.Context, id string) (*view.Ruleset, error)
	GetRulesetData(ctx context.Context, id string) ([]byte, string, error)
	GetActivationHistory(ctx context.Context, id string) ([]view.ActivationRecord, error)
	DeleteRuleset(ctx context.Context, id string) error
}

func NewRulesetService(rulesetRepository repository.RulesetRepository) RulesetService {
	return &rulesetServiceImpl{rulesetRepository: rulesetRepository}
}

type rulesetServiceImpl struct {
	rulesetRepository repository.RulesetRepository
}

func (r rulesetServiceImpl) CreateRuleset(ctx context.Context, name string, apiType view.ApiType, linter string, filename string, data []byte) (*view.Ruleset, error) {
	userId := secctx.GetUserId(ctx)

	// TODO: check for duplicate names!

	ent := entity.RulesetWithData{
		Ruleset: entity.Ruleset{
			Id:           uuid.NewString(),
			Name:         name,
			Status:       view.RulesetStatusInactive,
			CreatedAt:    time.Now(),
			CreatedBy:    userId,
			ApiType:      apiType,
			Linter:       view.Linter(linter),
			FileName:     filename,
			CanBeDeleted: true,
		},
		Data: data,
	}
	err := r.rulesetRepository.CreateRuleset(ctx, ent)
	if err != nil {
		return nil, err
	}

	resEnt, err := r.rulesetRepository.GetRulesetById(ctx, ent.Id)
	if err != nil {
		return nil, err
	}
	if resEnt == nil {
		return nil, fmt.Errorf("new ruleset not found")
	}
	result := entity.MakeRulesetView(*resEnt)
	return &result, nil
}

func (r rulesetServiceImpl) ActivateRuleset(ctx context.Context, id string) error {
	rsToActivate, err := r.rulesetRepository.GetRulesetById(ctx, id)
	if err != nil {
		return err
	}
	if rsToActivate == nil {
		return fmt.Errorf("ruleset with id %s not found", id)
	}

	currentRs, err := r.rulesetRepository.GetActiveRulesets(ctx, rsToActivate.ApiType)
	if err != nil {
		return err
	}
	if len(currentRs) == 0 {
		return fmt.Errorf("current active rulesets for api type %s are not found", rsToActivate.ApiType)
	}

	currentR, exists := currentRs[rsToActivate.Linter]
	if !exists {
		return fmt.Errorf("current active ruleset for api type %s and linter %s is not found", rsToActivate.ApiType, rsToActivate.Linter)
	}

	return r.rulesetRepository.ActivateRuleset(ctx, id, currentR.Id)
}

func (r rulesetServiceImpl) ListRulesets(ctx context.Context) ([]view.Ruleset, error) {
	ents, err := r.rulesetRepository.ListRulesets(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]view.Ruleset, 0)

	// TODO: need to sort the rulesets properly!
	/*
	   The sorting follows a strict order:
	     1. The currently active ruleset (always appears first).
	     2. Rulesets that have never been activated (in order of creation).
	     3. All other rulesets, sorted by the latest activation date (most recent first).
	*/

	for _, ent := range ents {
		result = append(result, entity.MakeRulesetView(ent))
	}

	return result, nil
}

func (r rulesetServiceImpl) GetRuleset(ctx context.Context, id string) (*view.Ruleset, error) {
	ent, err := r.rulesetRepository.GetRulesetById(ctx, id)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, nil
	}
	result := entity.MakeRulesetView(*ent)
	return &result, nil
}

func (r rulesetServiceImpl) GetRulesetData(ctx context.Context, id string) ([]byte, string, error) {
	ent, err := r.rulesetRepository.GetRulesetWithData(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if ent == nil {
		return nil, "", nil
	}
	return ent.Data, ent.FileName, nil
}

func (r rulesetServiceImpl) GetActivationHistory(ctx context.Context, id string) ([]view.ActivationRecord, error) {
	ent, err := r.rulesetRepository.GetRulesetById(ctx, id)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, &exception.CustomError{
			Status:  http.StatusNotFound,
			Code:    exception.EntityNotFound,
			Message: exception.EntityNotFoundMsg,
			Params:  map[string]interface{}{"entity": "ruleset", "id": id},
		}
	}

	ents, err := r.rulesetRepository.GetActivationHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]view.ActivationRecord, 0)
	for _, ent := range ents {
		result = append(result, entity.MakeActivationRecordView(ent))
	}
	return result, nil
}

func (r rulesetServiceImpl) DeleteRuleset(ctx context.Context, id string) error {
	ent, err := r.rulesetRepository.GetRulesetById(ctx, id)
	if err != nil {
		return err
	}
	if ent == nil {
		return &exception.CustomError{
			Status:  http.StatusNotFound,
			Code:    exception.EntityNotFound,
			Message: exception.EntityNotFoundMsg,
			Params:  map[string]interface{}{"entity": "ruleset", "id": id},
		}
	}
	if !ent.CanBeDeleted {
		return &exception.CustomError{
			Status:  http.StatusBadRequest,
			Code:    exception.RulesetCanNotBeDeleted,
			Message: exception.RulesetCanNotBeDeletedMsg,
			Params:  map[string]interface{}{"id": id},
		}
	}

	return r.rulesetRepository.DeleteRuleset(ctx, id)
}
