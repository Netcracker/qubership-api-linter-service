package entity

import "time"

type LintedVersion struct {
	tableName struct{} `pg:"linted_version"`

	PackageId   string    `pg:"package_id,pk,type:varchar,notnull"`
	Version     string    `pg:"version,pk,type:varchar,notnull"`
	Revision    int       `pg:"revision,pk,type:integer,notnull"`
	LintStatus  string    `pg:"lint_status,type:varchar,notnull"`
	LintDetails string    `pg:"lint_details,type:varchar"`
	LintedAt    time.Time `pg:"linted_at,type:timestamp without time zone,notnull"`
}
