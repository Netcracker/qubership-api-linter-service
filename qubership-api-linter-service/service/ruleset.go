package service

import (
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/view"
)

type RulesetService interface {
	GetRuleset(id string) (*view.Ruleset, error)
}

func NewRulesetService(rulesetRepository repository.RulesetRepository) RulesetService {
	return &rulesetServiceImpl{rulesetRepository: rulesetRepository}
}

type rulesetServiceImpl struct {
	rulesetRepository repository.RulesetRepository
}

func (r rulesetServiceImpl) GetRuleset(id string) (*view.Ruleset, error) {
	ent, err := r.rulesetRepository.GetRulesetById(id)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, nil
	}
	result := entity.MakeRulesetView(*ent)
	return &result, nil
}
