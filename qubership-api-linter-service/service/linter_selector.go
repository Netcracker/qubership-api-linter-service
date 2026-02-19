package service

import (
	"context"

	"github.com/Netcracker/qubership-api-linter-service/repository"
	"github.com/Netcracker/qubership-api-linter-service/view"
)

type LinterSelectorService interface {
	SelectLintersAndRuleset(ctx context.Context, t view.ApiType) []view.LinterAndRuleset
}

type linterSelectorServiceImpl struct {
	repo              repository.RulesetRepository
	systemInfoService SystemInfoService
}

func (l linterSelectorServiceImpl) SelectLintersAndRuleset(ctx context.Context, t view.ApiType) []view.LinterAndRuleset {
	rulesets, err := l.repo.GetActiveRulesets(ctx, t)
	if err != nil {
		return []view.LinterAndRuleset{{
			Linter:    view.UnknownLinter,
			RulesetId: "",
			Err:       err,
		}}
	}

	result := make([]view.LinterAndRuleset, 0)

	switch t {
	case view.OpenAPI31Type, view.OpenAPI30Type, view.OpenAPI20Type:
		rs, exists := rulesets[view.SpectralLinter]
		spectralRsId := ""
		if exists {
			spectralRsId = rs.Id
		}
		result = append(result, view.LinterAndRuleset{
			Linter:    view.SpectralLinter,
			RulesetId: spectralRsId,
			Err:       nil,
		})
		if l.systemInfoService.IsAiOasLinterEnabled() {
			rs, exists = rulesets[view.AiOasLinter]
			AiOasRsId := ""
			if exists {
				AiOasRsId = rs.Id
			}
			result = append(result, view.LinterAndRuleset{
				Linter:    view.AiOasLinter,
				RulesetId: AiOasRsId,
				Err:       nil,
			})
		}
		break
	case view.AsyncAPI30Type:
		rs, exists := rulesets[view.SpectralLinter]
		spectralRsId := ""
		if exists {
			spectralRsId = rs.Id
		}
		result = append(result, view.LinterAndRuleset{
			Linter:    view.SpectralAsyncLinter,
			RulesetId: spectralRsId,
			Err:       nil,
		})
		break
	default:
		// lint of this type is not supported now
		result = append(result, view.LinterAndRuleset{
			Linter:    view.UnknownLinter,
			RulesetId: "",
			Err:       nil,
		})
		break
	}

	return result
}

func NewLinterSelectorService(repo repository.RulesetRepository, systemInfoService SystemInfoService) LinterSelectorService {
	return &linterSelectorServiceImpl{
		repo:              repo,
		systemInfoService: systemInfoService,
	}
}
