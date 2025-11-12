create table linted_operation
(
    package_id         varchar not null,
    version            varchar not null,
    revision           integer not null,
    file_id            varchar not null,
    operation_id       varchar not null,
    slug               varchar not null,
    specification_type varchar not null,
    ruleset_id         varchar not null,
    data_hash          varchar,
    lint_status        varchar not null,
    lint_details       varchar,

    constraint linted_operation_pk
        primary key (package_id, version, revision, file_id, operation_id),
    constraint linted_operation_ruleset_fk
        foreign key (ruleset_id) references ruleset (id)
);

create index linted_operation_package_id_version_revision_slug_index
    on linted_operation (package_id, version, revision, slug);

create table lint_operation_result
(
    data_hash      varchar not null,
    ruleset_id     varchar not null
        constraint lint_operation_result_ruleset_id_fk
            references ruleset (id),
    linter_version varchar not null,
    data           bytea   not null,
    summary        jsonb   not null,
    constraint lint_operation_result_pk
        primary key (data_hash, ruleset_id)
);

