package service

import (
	"github.com/Netcracker/qubership-api-linter-service/view"
)

func calculateOperationScore(spectralSummary view.SpectralResultSummary, aiProblems []view.AIApiDocCatProblem) int {
	errorCount := 0
	warningCount := 0
	infoCount := 0
	for _, problem := range aiProblems {
		switch problem.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}
	if spectralSummary.ErrorCount > 0 || errorCount > 0 {
		return 0
	}
	if spectralSummary.WarningCount > 0 || warningCount > 0 {
		return 1
	}
	if spectralSummary.InfoCount > 0 || (infoCount > 0) {
		return 2
	}
	return 3
}

func calculateVersionScore(numberOfOperations int, operationsScores []view.Score) view.Score {
	result := view.Score{
		Details: []view.ScoreDetail{},
	}

	if numberOfOperations == 0 || len(operationsScores) == 0 {
		return result
	}

	// Count (good + acceptable) operations
	goodOrAcceptable := 0
	hasAcceptable := false
	hasBad := false
	detailMap := make(map[view.ScoreName]view.Grade)

	for _, score := range operationsScores {
		switch score.OverallScore {
		case view.Good:
			goodOrAcceptable++
		case view.Acceptable:
			goodOrAcceptable++
			hasAcceptable = true
		case view.Bad:
			hasBad = true
		}

		// Aggregate Details: take worst grade for each ScoreName
		for _, detail := range score.Details {
			current, exists := detailMap[detail.Name]
			if !exists || isWorseGrade(detail.Value, current) {
				detailMap[detail.Name] = detail.Value
			}
		}
	}

	// Calculate percentage of operations with Good or Acceptable grades
	percentage := goodOrAcceptable * 100 / numberOfOperations
	result.DigitalScore = percentage

	// Determine OverallScore based on percentage
	// (good + acceptable) < 50% -> bad
	// 50% <= (good + acceptable) < 70% -> acceptable
	// (good + acceptable) >= 70% -> acceptable if (hasAcceptable || hasBad) or good
	switch {
	case percentage < 50:
		result.OverallScore = view.Bad
	case percentage < 70:
		result.OverallScore = view.Acceptable
	default:
		if hasAcceptable || hasBad {
			result.OverallScore = view.Acceptable
		} else {
			result.OverallScore = view.Good
		}
	}

	// Convert detailMap back to slice
	result.Details = make([]view.ScoreDetail, 0, len(detailMap))
	for name, grade := range detailMap {
		result.Details = append(result.Details, view.ScoreDetail{
			Name:  name,
			Value: grade,
		})
	}

	return result
}
