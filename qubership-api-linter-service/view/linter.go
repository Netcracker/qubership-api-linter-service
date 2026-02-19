package view

type Linter string

const (
	SpectralLinter      Linter = "spectral"
	AiOasLinter         Linter = "ai_oas"
	SpectralAsyncLinter Linter = "spectral_asyncapi"

	UnknownLinter Linter = "unknown"
)

type LinterAndRuleset struct {
	Linter    Linter
	RulesetId string
	Err       error
}
