package view

type TaskStatus string

const (
	StatusNotStarted     TaskStatus = "not_started"
	StatusProcessing     TaskStatus = "processing" // todo version only?
	StatusWaitingForDocs TaskStatus = "waiting_for_docs"
	StatusLinting        TaskStatus = "linting" // docs only
	StatusComplete       TaskStatus = "complete"
	StatusError          TaskStatus = "error"
)
