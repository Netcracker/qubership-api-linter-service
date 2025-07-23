package view

type TaskStatus string

const (
	TaskStatusNotStarted     TaskStatus = "not_started"
	TaskStatusProcessing     TaskStatus = "processing" // todo version only?
	TaskStatusWaitingForDocs TaskStatus = "waiting_for_docs"
	TaskStatusLinting        TaskStatus = "linting" // docs only
	TaskStatusComplete       TaskStatus = "complete"
	TaskStatusError          TaskStatus = "error"
)
