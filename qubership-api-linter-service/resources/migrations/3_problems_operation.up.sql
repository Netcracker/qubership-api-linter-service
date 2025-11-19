create table problems_operation
(
    package_id   varchar not null,
    version      varchar not null,
    revision     integer not null,
    operation_id varchar not null,
    prompt_hash  varchar not null,
    problems     jsonb   not null,

    constraint problems_operation_pk
        primary key (package_id, version, revision, operation_id)
);

create index problems_operation_package_id_version_revision_index
    on problems_operation (package_id, version, revision);

