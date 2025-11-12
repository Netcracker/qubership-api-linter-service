package entity

import "github.com/Netcracker/qubership-api-linter-service/view"

type LintedOperation struct {
	tableName struct{} `pg:"linted_operation"`

	PackageId         string                    `pg:"package_id,pk,type:varchar,notnull"`
	Version           string                    `pg:"version,pk,type:varchar,notnull"`
	Revision          int                       `pg:"revision,pk,type:integer,notnull"`
	FileId            string                    `pg:"file_id,pk,type:varchar,notnull"`
	OperationId       string                    `pg:"operation_id,pk,type:varchar,notnull"`
	Slug              string                    `pg:"slug,type:varchar,notnull"`
	SpecificationType view.ApiType              `pg:"specification_type,type:varchar,notnull"`
	RulesetId         string                    `pg:"ruleset_id,type:varchar,notnull"`
	DataHash          string                    `pg:"data_hash,type:varchar"`
	LintStatus        view.LintedDocumentStatus `pg:"lint_status,type:varchar,notnull"`
	LintDetails       string                    `pg:"lint_details,type:varchar"`
}

type LintOperationResult struct {
	tableName struct{} `pg:"lint_operation_result"`

	DataHash      string                 `pg:"data_hash,pk,type:varchar,notnull"`
	RulesetId     string                 `pg:"ruleset_id,pk,type:varchar,notnull"`
	LinterVersion string                 `pg:"linter_version,type:varchar,notnull"`
	Data          []byte                 `pg:"data,type:bytea,notnull"`
	Summary       map[string]interface{} `pg:"summary,type:jsonb,notnull"`
}
