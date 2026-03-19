package view

type VersionScore struct {
	Status  ScoringStatus  `json:"status"`
	Reason  string         `json:"reason"`
	Debug   string         `json:"debug"`
	Details ScoringDetails `json:"details"`
}
type ScoringStatus string

const (
	ScoringPassed            ScoringStatus = "passed"
	ScoringPassedWithDefects ScoringStatus = "passed_with_defects"
	ScoringBlocked           ScoringStatus = "blocked"
)

type ScoringDetails struct {
	BackwardsCompatibility BackwardsCompatibilityDetails `json:"backwards_compatibility"`
	QualityCheck           []ValidationDetails           `json:"quality_check"`
}

type BackwardsCompatibilityDetails struct {
	Status ScoringStatus `json:"status"`
	Reason string        `json:"reason"`

	// TODO: split data by API types???

	// breaking changes
	Breaking int `json:"breaking"`

	// Issues for audience
	BreakingInternal int `json:"breaking_internal"`
	BreakingExternal int `json:"breaking_external"`
	BreakingUnknown  int `json:"breaking_unknown"`

	// Audience transition
	Internal2Unknown  int `json:"internal_2_unknown"`
	External2Internal int `json:"external_2_internal"`
	External2Unknown  int `json:"external_2_unknown"`

	InternalErrors []string `json:"internal_errors,omitempty"`
}

type ValidationDetails struct {
	Linter         Linter        `json:"linter"`
	Status         ScoringStatus `json:"status"`
	Reason         string        `json:"reason"`
	ErrorsCount    int           `json:"errors_count"`
	WarningsCount  int           `json:"warnings_count"`
	InternalErrors []string      `json:"internal_errors,omitempty"`
}
