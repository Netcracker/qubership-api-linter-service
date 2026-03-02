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

type InternalLinterConfig struct {
	Linter               Linter
	ApiTypes             []ApiType
	DisplayName          string
	Enabled              bool
	Workers              int
	IncludePackages      []string
	ExcludePackages      []string
	LinterSpecificParams map[string]interface{}
}

type ExternalLinterConfig struct {
	Linter      Linter    `json:"linter"`
	ApiTypes    []ApiType `json:"apiTypes"`
	DisplayName string    `json:"displayName"`
	Enabled     bool      `json:"enabled"`
}
