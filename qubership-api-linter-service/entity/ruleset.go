package entity

import "time"

type Ruleset struct {
	tableName struct{} `pg:"ruleset"`

	Id        string    `pg:"id,pk,type:varchar"`
	Name      string    `pg:"name,type:varchar,notnull"`
	Status    string    `pg:"status,type:varchar,notnull"`
	Data      []byte    `pg:"data,type:bytea,notnull"`
	CreatedAt time.Time `pg:"created_at,type:timestamp without time zone,notnull"`
	CreatedBy string    `pg:"created_by,type:varchar"`
	DeletedAt time.Time `pg:"deleted_at,type:timestamp without time zone"`
	DeletedBy string    `pg:"deleted_by,type:varchar,notnull"`
	ApiType   string    `pg:"api_type,type:varchar,notnull"`
	Linter    string    `pg:"linter,type:varchar,notnull"`
}

type RulesetActivationHistory struct {
	tableName struct{} `pg:"ruleset_activation_history"`

	RulesetId   string    `pg:"ruleset_id,type:varchar,notnull"`
	ActivatedAt time.Time `pg:"activated_at,type:timestamp without time zone"`
	ActivatedBy string    `pg:"activated_by,type:varchar"`
}
