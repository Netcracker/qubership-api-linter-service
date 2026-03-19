package entity

import (
	"time"

	"github.com/Netcracker/qubership-api-linter-service/view"
)

type VersionScore struct {
	tableName struct{} `pg:"version_score"`

	PackageId string `pg:"package_id,pk,type:varchar,notnull"`
	Version   string `pg:"version,pk,type:varchar,notnull"`
	Revision  int    `pg:"revision,pk,type:integer,notnull"`

	ScoredAt time.Time `pg:"scored_at,type:timestamp without time zone,notnull"`

	Status view.ScoringStatus `pg:"status,type:varchar,notnull"`
	Reason string             `pg:"reason,type:varchar"`
	Debug  string             `pg:"debug,type:varchar"`

	Details view.ScoringDetails `pg:"details,type:jsonb"`
}
