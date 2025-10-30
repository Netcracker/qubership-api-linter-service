package view

import "time"

type PublishedVersionSourceDataConfig struct {
	Sources []byte      `json:"sources"`
	Config  BuildConfig `json:"config"`
}

type BuildConfig struct {
	PackageId                    string                  `json:"packageId"`
	Version                      string                  `json:"version"`
	BuildType                    BuildType               `json:"buildType"`
	PreviousVersion              string                  `json:"previousVersion"`
	PreviousVersionPackageId     string                  `json:"previousVersionPackageId"`
	Status                       string                  `json:"status"`
	Refs                         []BCRef                 `json:"refs,omitempty"`
	Files                        []BCFile                `json:"files,omitempty"`
	PublishId                    string                  `json:"publishId"`
	Metadata                     BuildConfigMetadata     `json:"metadata,omitempty"`
	CreatedBy                    string                  `json:"createdBy"`
	NoChangelog                  bool                    `json:"noChangeLog,omitempty"`    // for migration
	PublishedAt                  time.Time               `json:"publishedAt,omitempty"`    // for migration
	MigrationBuild               bool                    `json:"migrationBuild,omitempty"` //for migration
	MigrationId                  string                  `json:"migrationId,omitempty"`    //for migration
	ComparisonRevision           int                     `json:"comparisonRevision,omitempty"`
	ComparisonPrevRevision       int                     `json:"comparisonPrevRevision,omitempty"`
	UnresolvedRefs               bool                    `json:"unresolvedRefs,omitempty"`
	ResolveRefs                  bool                    `json:"resolveRefs,omitempty"`
	ResolveConflicts             bool                    `json:"resolveConflicts,omitempty"`
	ServiceName                  string                  `json:"serviceName,omitempty"`
	ApiType                      string                  `json:"apiType,omitempty"`   //for operation group
	GroupName                    string                  `json:"groupName,omitempty"` //for operation group
	Format                       string                  `json:"format,omitempty"`    //for operation group
	ExternalMetadata             map[string]interface{}  `json:"externalMetadata,omitempty"`
	ValidationRulesSeverity      ValidationRulesSeverity `json:"validationRulesSeverity,omitempty"`
	AllowedOasExtensions         *[]string               `json:"allowedOasExtensions,omitempty"`         // for export
	DocumentId                   string                  `json:"documentId,omitempty"`                   // for export
	OperationsSpecTransformation string                  `json:"operationsSpecTransformation,omitempty"` // for export
}

type BCRef struct {
	RefId         string `json:"refId"`
	Version       string `json:"version"` //format: version@revision
	ParentRefId   string `json:"parentRefId"`
	ParentVersion string `json:"parentVersion"` //format: version@revision
	Excluded      bool   `json:"excluded,omitempty"`
}

type BCFile struct {
	FileId   string   `json:"fileId"`
	Slug     string   `json:"slug"`  //for migration
	Index    int      `json:"index"` //for migration
	Publish  *bool    `json:"publish"`
	Labels   []string `json:"labels"`
	BlobId   string   `json:"blobId,omitempty"`
	XApiKind string   `json:"xApiKind,omitempty"`
}

type BuildType string

const ChangelogType BuildType = "changelog"
const PublishType BuildType = "build"
const DocumentGroupType_deprecated BuildType = "documentGroup"
const ReducedSourceSpecificationsType_deprecated BuildType = "reducedSourceSpecifications"
const MergedSpecificationType_deprecated BuildType = "mergedSpecification"

const ExportVersion BuildType = "exportVersion"
const ExportRestDocument BuildType = "exportRestDocument"
const ExportRestOperationsGroup BuildType = "exportRestOperationsGroup"

type BuildConfigMetadata struct {
	BranchName    string   `json:"branchName,omitempty"`
	RepositoryUrl string   `json:"repositoryUrl,omitempty"`
	CloudName     string   `json:"cloudName,omitempty"`
	CloudUrl      string   `json:"cloudUrl,omitempty"`
	Namespace     string   `json:"namespace,omitempty"`
	VersionLabels []string `json:"versionLabels,omitempty"`
}

type ValidationRulesSeverity struct {
	BrokenRefs string `json:"brokenRefs"`
}
