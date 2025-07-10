package repository

import (
	"errors"
	"github.com/Netcracker/qubership-api-linter-service/db"
	"github.com/Netcracker/qubership-api-linter-service/entity"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/go-pg/pg/v10"
)

type RuleSetRepository interface {
	//list rulesets

	GetActiveRulesets(apiType view.ApiType) (map[view.Linter]entity.Ruleset, error)
	GetRulesetById(id string) (*entity.Ruleset, error)
}

type ruleSetRepositoryImpl struct {
	cp db.ConnectionProvider
}

func (r ruleSetRepositoryImpl) GetRulesetById(id string) (*entity.Ruleset, error) {
	var ruleset entity.Ruleset

	err := r.cp.GetConnection().Model(&ruleset).Where("id = ?", id).Select()
	if err != nil {
		return nil, err
	}
	return &ruleset, nil
}

func NewRuleSetRepository(cp db.ConnectionProvider) RuleSetRepository {
	return ruleSetRepositoryImpl{cp: cp}
}

func (r ruleSetRepositoryImpl) GetActiveRulesets(apiType view.ApiType) (map[view.Linter]entity.Ruleset, error) {
	var rulesets []entity.Ruleset

	// TODO: do we need content in this case???

	err := r.cp.GetConnection().Model(&rulesets).Where("api_type=?", apiType).Where("status=?", "active").Select()
	if err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	result := make(map[view.Linter]entity.Ruleset)

	for _, rs := range rulesets {
		result[view.Linter(rs.Linter)] = rs
	}

	return result, nil
}
