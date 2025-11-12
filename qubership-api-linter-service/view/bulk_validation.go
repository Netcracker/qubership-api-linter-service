package view

type BulkValidationRequest struct {
	PackageId       string   `json:"packageId"`
	Version         string   `json:"version,omitempty"`
	ExcludePackages []string `json:"excludePackages,omitempty"`
}

type BulkValidationStartResponse struct {
	JobId string `json:"jobId"`
}

type BulkValidationStatusResponse struct {
	JobId             string                `json:"jobId"`
	Status            AsyncStatus           `json:"status"`
	ProcessedVersions int                   `json:"processedVersions"`
	TotalVersions     int                   `json:"totalVersions"`
	Error             string                `json:"error,omitempty"`
	Packages          []BulkValidationEntry `json:"packages,omitempty"`
}

type BulkValidationEntry struct {
	PackageId         string `json:"packageId"`
	Version           string `json:"version"`
	ValidationTaskId  string `json:"validationTaskId,omitempty"`
	ValidationStarted bool   `json:"validationStarted"`
}

type PackageVersionsResponse struct {
	Versions []PackageVersion `json:"versions"`
}

type PackageVersion struct {
	Version string `json:"version"`
	Status  string `json:"status"`
}
