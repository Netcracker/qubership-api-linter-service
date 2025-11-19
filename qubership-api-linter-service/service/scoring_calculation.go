package service

import (
	"math"

	"github.com/Netcracker/qubership-api-linter-service/view"
)

func CalculateScore(spectralSummary view.SpectralResultSummary, aiProblems []view.AIApiDocCatProblem) int {
	// Calculate score for Spectral results (0-50 points)
	spectralScore := calculateSpectralScore(spectralSummary)

	// Calculate score for AI API Doc problems (0-50 points)
	aiScore := calculateAIScore(aiProblems)

	// Combine both scores (0-100)
	totalScore := spectralScore + aiScore

	// Ensure score is within 0-100 range
	return clamp(totalScore, 0, 100)
}

func calculateSpectralScore(summary view.SpectralResultSummary) int {
	// Assign weights to different issue types
	errorWeight := 10.0  // Most severe
	warningWeight := 5.0 // Medium severity
	infoWeight := 2.0    // Low severity
	hintWeight := 1.0    // Least severe

	// Calculate total penalty points
	totalPenalty := float64(summary.ErrorCount)*errorWeight +
		float64(summary.WarningCount)*warningWeight +
		float64(summary.InfoCount)*infoWeight +
		float64(summary.HintCount)*hintWeight

	// Normalize to 0-50 scale using a logarithmic scale to handle wide ranges
	// Higher penalty = lower score
	maxExpectedPenalty := 100.0 // Adjust this based on expected maximum issues

	if totalPenalty == 0 {
		return 50 // Perfect score for this part
	}

	// Use logarithmic scaling to handle large numbers of issues gracefully
	normalizedPenalty := math.Log1p(totalPenalty) / math.Log1p(maxExpectedPenalty)

	// Convert penalty to score (0-50)
	score := 50.0 * (1.0 - math.Min(normalizedPenalty, 1.0))

	return int(math.Round(score))
}

func calculateAIScore(problems []view.AIApiDocCatProblem) int {
	if len(problems) == 0 {
		return 50 // Perfect score for this part
	}

	// Count issues by severity
	errorCount := 0
	warningCount := 0
	infoCount := 0
	hintCount := 0

	for _, problem := range problems {
		switch problem.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		case "hint":
			hintCount++
		}
	}

	// Assign weights (same as spectral for consistency)
	errorWeight := 10.0
	warningWeight := 5.0
	infoWeight := 2.0
	hintWeight := 1.0

	// Calculate total penalty points
	totalPenalty := float64(errorCount)*errorWeight +
		float64(warningCount)*warningWeight +
		float64(infoCount)*infoWeight +
		float64(hintCount)*hintWeight

	// Normalize to 0-50 scale
	maxExpectedPenalty := 100.0

	if totalPenalty == 0 {
		return 50
	}

	// Use logarithmic scaling
	normalizedPenalty := math.Log1p(totalPenalty) / math.Log1p(maxExpectedPenalty)

	// Convert penalty to score (0-50)
	score := 50.0 * (1.0 - math.Min(normalizedPenalty, 1.0))

	return int(math.Round(score))
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Alternative simpler implementation if you prefer a linear approach
func CalculateScoreSimple(spectralSummary view.SpectralResultSummary, aiProblems []view.AIApiDocCatProblem) int {
	// Simple linear scoring approach

	// Calculate penalty for spectral results
	spectralPenalty := spectralSummary.ErrorCount*10 +
		spectralSummary.WarningCount*5 +
		spectralSummary.InfoCount*2 +
		spectralSummary.HintCount*1

	// Calculate penalty for AI problems
	aiPenalty := 0
	for _, problem := range aiProblems {
		switch problem.Severity {
		case "error":
			aiPenalty += 10
		case "warning":
			aiPenalty += 5
		case "info":
			aiPenalty += 2
		case "hint":
			aiPenalty += 1
		}
	}

	// Normalize both penalties to 0-50 scale
	maxPenalty := 200 // Adjust based on what you consider "worst case"

	spectralScore := 50 - clamp((spectralPenalty*50)/maxPenalty, 0, 50)
	aiScore := 50 - clamp((aiPenalty*50)/maxPenalty, 0, 50)

	// Combine scores
	totalScore := spectralScore + aiScore

	return clamp(totalScore, 0, 100)
}
