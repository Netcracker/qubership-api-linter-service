create table scoring_operation
(
    package_id   varchar not null,
    version      varchar not null,
    revision     integer not null,
    operation_id varchar not null,
    score        jsonb   not null,

    constraint scoring_operation_pk
        primary key (package_id, version, revision, operation_id)
);

create index scoring_operation_package_id_version_revision_index
    on scoring_operation (package_id, version, revision);

