package view

type IAProblemsOutput struct {
	Problems []AIApiDocProblem `json:"problems"`
}

type AIApiDocProblem struct {
	Severity string `json:"severity" jsonschema:"enum=error,enum=warning,enum=info"`
	Text     string `json:"text"`
}

type AIApiDocCatProblemsOutput struct {
	Problems []AIApiDocCatProblem `json:"problems"`
}

type AIApiDocCatProblem struct {
	Severity string `json:"severity"`
	Text     string `json:"text"`
	Category string `json:"category"`
}

type ProblemSeverity string

const PSError = "error"
const PSWarning = "warning"
const PSInfo = "info"

type ProblemCategories struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}
