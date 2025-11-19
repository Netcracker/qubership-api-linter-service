package view

type OperationResult struct {
	Issues             []ValidationIssue  `json:"issues"`
	ValidatedOperation ValidatedOperation `json:"operation"`
}

type ValidatedOperation struct {
	DocSlug string `json:"docSlug"`

	OperationId string `json:"operationId"`
	Title       string `json:"title"`
	Path        string `json:"path"`   // TODO: REST only
	Method      string `json:"method"` // TODO: REST only

	ApiType ApiType `json:"specificationType"`
	DocName string  `json:"documentName"`
}
