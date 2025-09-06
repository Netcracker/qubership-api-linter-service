package view

type DocumentResult struct {
	Ruleset           Ruleset           `json:"ruleset"`
	Issues            interface{}       `json:"issues"`
	ValidatedDocument ValidatedDocument `json:"document"`
}
