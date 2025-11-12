package view

type EnhancementStatusResponse struct {
	Status  AsyncStatus `json:"status"`
	Details string      `json:"details"`
}

type AsyncStatus string

const ESNotStarted AsyncStatus = "not_started"
const ESProcessing AsyncStatus = "processing"
const ESSuccess AsyncStatus = "success"
const ESError AsyncStatus = "error"

type PublishEnhancementRequest struct {
	PackageId       string        `json:"packageId"`
	Version         string        `json:"version"`
	PreviousVersion string        `json:"previousVersion"`
	Status          VersionStatus `json:"status"`
	Labels          []string      `json:"labels"`
}

type PublishResponse struct {
	PublishId string `json:"publishId"`
}

type VersionStatus string

const (
	Draft   VersionStatus = "draft"
	Release VersionStatus = "release"
)
