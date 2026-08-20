drop index if exists version_lint_task_error_kind_last_active_index;

alter table version_lint_task drop column error_kind;

alter table version_lint_task drop column validation_retry_count;

alter table document_lint_task drop column error_kind;
