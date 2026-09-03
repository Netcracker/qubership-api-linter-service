alter table version_lint_task add column error_kind varchar;

alter table version_lint_task add column validation_retry_count integer default 0 not null;

alter table document_lint_task add column error_kind varchar;

create index version_lint_task_error_kind_last_active_index
    on version_lint_task (error_kind, last_active);
