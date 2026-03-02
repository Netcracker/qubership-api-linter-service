alter table linted_document
    drop constraint linted_document_pk;

alter table linted_document
    add constraint linted_document_pk
        primary key (package_id, version, revision, file_id, ruleset_id);

insert into ruleset
values ('c6a9b817-10c7-4f59-8a2b-8f3eebe95929', 'default-ai-openapi-2-0', 'active',
        'prompt: "Validate descriptions in the openapi specification. Check the presence, quality, and usefulness."'::BYTEA, now(), 'system', 'openapi-2-0', 'ai_linter',
        'default-openapi-2-0.yaml', false);
insert into ruleset
values ('d417c9c7-77d4-4f8a-b4a1-6231a1d8a885', 'default-ai-openapi-3-0', 'active',
        'prompt: "Validate descriptions in the openapi specification. Check the presence, quality, and usefulness."'::BYTEA, now(), 'system', 'openapi-3-0', 'ai_linter',
        'default-openapi-3-0.yaml', false);
insert into ruleset
values ('de66b276-b5b2-416f-a94b-5b01e7d40c84', 'default-ai-openapi-3-1', 'active',
        'prompt: "Validate descriptions in the openapi specification. Check the presence, quality, and usefulness."'::BYTEA, now(), 'system', 'openapi-3-1', 'ai_linter',
        'default-openapi-3-1.yaml', false);

insert into ruleset
values ('e5142acd-82ee-4e69-abe6-f1a2db330b0e', 'default-asyncapi-3-0', 'active',
        'extends: "spectral:asyncapi"'::BYTEA, now(), 'system', 'asyncapi-3-0', 'spectral',
        'default-asyncapi-3-0.yaml', false);