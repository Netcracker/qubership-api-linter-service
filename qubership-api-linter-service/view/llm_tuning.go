package view

type UpdatePromptReq struct {
	Prompt string `json:"prompt"`
}

type UpdateModelReq struct {
	Model string `json:"model"`
}
