package entity

import "github.com/Netcracker/qubership-api-linter-service/view"

type OperationScore struct {
	tableName struct{} `pg:"scoring_operation,alias:scoring_operation"`

	PackageId   string `pg:"package_id,pk,type:varchar,notnull"`
	Version     string `pg:"version,pk,type:varchar,notnull"`
	Revision    int    `pg:"revision,pk,type:integer,notnull"`
	OperationId string `pg:"operation_id,pk,type:varchar,notnull"`

	Score view.Score `pg:"score,type:jsonb"`
}
