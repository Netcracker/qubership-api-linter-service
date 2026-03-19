create table version_score
(
    package_id varchar                     not null,
    version    varchar                     not null,
    revision   integer                     not null,
    scored_at  timestamp without time zone not null,
    status     varchar                     not null,
    reason     varchar,
    debug     varchar,
    details    jsonb,
    constraint version_score_pk
        primary key (package_id, version, revision)
);
