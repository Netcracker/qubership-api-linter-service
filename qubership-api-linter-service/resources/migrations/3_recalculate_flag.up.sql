alter table version_lint_task
    add column recalculate boolean not null default false;

alter table document_lint_task
    add column recalculate boolean not null default false;
