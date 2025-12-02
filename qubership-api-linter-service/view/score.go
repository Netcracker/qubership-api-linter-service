package view

type Score struct {
	OverallScore Grade         `json:"overallScore"` // TODO: deprecated?
	DigitalScore int           `json:"digitalScore"`
	Details      []ScoreDetail `json:"details"`
}

type ScoreDetail struct {
	Name  ScoreName `json:"name"`
	Value Grade     `json:"value"`
}

type Grade string

const Good Grade = "Passed"
const Acceptable = "Passed conditionally (>50% && <70% passing)"
const Bad Grade = "Blocked"

type ScoreName string

const ScoreNameLint ScoreName = "Linter"
const ScoreNameProblems ScoreName = "LLM detected problems"
