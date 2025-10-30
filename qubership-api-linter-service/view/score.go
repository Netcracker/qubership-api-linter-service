package view

type Score struct {
	OverallScore Grade         `json:"overallScore"`
	Details      []ScoreDetail `json:"details"`
}

type ScoreDetail struct {
	Name  ScoreName `json:"name"`
	Value Grade     `json:"value"`
}

type Grade string

const Good Grade = "Good"
const Acceptable = "Acceptable"
const Bad Grade = "Bad"

type ScoreName string

const ScoreNameLint ScoreName = "Linter"
const ScoreNameProblems ScoreName = "LLM detected problems"
