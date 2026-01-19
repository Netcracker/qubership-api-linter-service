package entity

import "github.com/Netcracker/qubership-api-linter-service/view"

type OperationProblems struct {
	tableName struct{} `pg:"problems_operation"`

	PackageId   string `pg:"package_id,pk,type:varchar,notnull"`
	Version     string `pg:"version,pk,type:varchar,notnull"`
	Revision    int    `pg:"revision,pk,type:integer,notnull"`
	OperationId string `pg:"operation_id,pk,type:varchar,notnull"`
	FileSlug    string `pg:"file_slug,type:varchar,notnull"`

	PromptHash string `pg:"prompt_hash,type:varchar,notnull"`

	Problems []view.AIApiDocCatProblem `pg:"problems,type:jsonb"`
}
