package view

type EnhancementStatusResponse struct {
	Status  EnhancementStatus `json:"status"`
	Details string            `json:"details"`
}

type EnhancementStatus string

const ESNotStarted EnhancementStatus = "not_started"
const ESProcessing EnhancementStatus = "processing"
const ESSuccess EnhancementStatus = "success"
const ESError EnhancementStatus = "error"

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
