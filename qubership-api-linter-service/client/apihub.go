package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"net/http"
	"net/url"
	"strconv"

	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/resty.v1"
)

type ApihubClient interface {
	GetRsaPublicKey(ctx context.Context) (*view.PublicKey, error)
	GetApiKeyByKey(apiKey string) (*view.ApihubApiKeyView, error)

	GetVersion(ctx context.Context, id, version string) (*view.VersionContent, error)

	GetVersionDocuments(ctx context.Context, packageId, version string) (*view.VersionDocuments, error)
	GetDocumentRawData(ctx context.Context, packageId, version string, fileId string) ([]byte, error)

	CheckAuthToken(ctx context.Context, token string) (bool, error)

	/*GetPackageByServiceName(ctx secctx.SecurityContext, workspaceId string, serviceName string) (*view.SimplePackage, error)
	GetPackageIdByServiceName(ctx secctx.SecurityContext, workspaceId string, serviceName string) (string, string, error)
	GetPackageById(ctx secctx.SecurityContext, id string) (*view.SimplePackage, error)
	CreatePackage(ctx secctx.SecurityContext, pkg view.PackageCreateRequest) (string, error)
	GetPackages(ctx secctx.SecurityContext, searchReq view.PackagesSearchReq) (*view.SimplePackages, error)
	PatchPackage(ctx secctx.SecurityContext, packageId string, patchReq view.PackagePatchReq) (*view.SimplePackage, error)
	GetVersionChanges(ctx secctx.SecurityContext, packageId string, version string, prevVersionPackageId string, prevVersion string, apiType string, limit int, page int) (*view.VersionChangesView, error)
	GetVersions(ctx secctx.SecurityContext, packageId string, searchReq view.VersionSearchRequest) (*view.PublishedVersionsView, error)
	GetVersionInfo(ctx secctx.SecurityContext, id string, version string) (*view.VersionContent, error)
	GetVersion(ctx secctx.SecurityContext, id, version string) (*view.VersionContent, error)
	GetVersionRevisions(ctx secctx.SecurityContext, id, version string, limit int, page int) (*view.VersionRevisionsView, error)
	GetVersionReferences(ctx secctx.SecurityContext, id, version string) (*view.VersionReferences, error)

	GetVersionSources(ctx secctx.SecurityContext, packageId, version string) (*view.PublishedVersionSourceDataConfig, error)
	GetVersionRestOperationsWithData(ctx secctx.SecurityContext, packageId string, version string, limit int, page int) (*view.RestOperations, error)
	GetOperationsList(ctx secctx.SecurityContext, packageId string, version string, apiType string, operationListReq view.OperationListRequest) (*view.CommonOperations, error)
	GetPackageActivityHistory(ctx secctx.SecurityContext, packageId string, parameters view.GetPackageActivityEventsReq) (*view.PackageActivityEvents, error)
	DeleteVersionsRecursively(packageId string, req view.DeleteVersionsRecursivelyReq) (string, error)

	Publish(ctx secctx.SecurityContext, config view.BuildConfig, src []byte, clientBuild bool, builderId string, saveSources bool, dependencies []string) (string, error)

	GetUserPackagesPromoteStatuses(ctx secctx.SecurityContext, packagesReq view.PackagesReq) (view.AvailablePackagePromoteStatuses, error)

	GetPublishStatus(ctx secctx.SecurityContext, packageId, publishId string) (view.StatusEnum, string, error)
	GetPublishStatuses(ctx secctx.SecurityContext, packageId string, publishIds []string) ([]view.PublishStatusResponse, error)

	GetAgent(ctx secctx.SecurityContext, agentId string) (*view.AgentInstance, error)
	GetAgents(ctx secctx.SecurityContext) ([]view.AgentInstance, error)

	GetApihubUrl() string

	GetSystemInfo(ctx secctx.SecurityContext) (*view.ApihubSystemInfo, error)

	GetUserById(ctx secctx.SecurityContext, userId string) (*view.User, error)

	GetApiKeyById(ctx secctx.SecurityContext, apiKeyId string) (*view.ApihubApiKeyView, error)

	ListCompletedActivities(offset int, limit int) ([]view.TransitionStatus, error)
	ListPackageTransitions() ([]view.PackageTransition, error)
	GetPublishedVersionsHistory(status string, publishedAfter *time.Time, publishedBefore *time.Time, limit int, page int) ([]view.PublishedVersionHistoryView, error)*/
}

func NewApihubClient(apihubUrl, accessToken string) ApihubClient {
	parsedApihubUrl, err := url.Parse(apihubUrl)
	apihubHost := ""
	if err != nil {
		log.Errorf("Can't parse apihub url: %v", err)
	} else {
		apihubHost = parsedApihubUrl.Hostname()
	}

	tr := http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	cl := http.Client{Transport: &tr, Timeout: time.Second * 60}
	client := resty.NewWithClient(&cl)
	if apihubHost != "" {
		client.SetRedirectPolicy(resty.DomainCheckRedirectPolicy(apihubHost))
	}

	return &apihubClientImpl{apihubUrl: apihubUrl, accessToken: accessToken, apiHubHost: apihubHost, client: client}
}

type apihubClientImpl struct {
	apihubUrl   string
	accessToken string
	apiHubHost  string
	client      *resty.Client
}

func (a apihubClientImpl) GetApiKeyByKey(apiKey string) (*view.ApihubApiKeyView, error) {
	tr := http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	cl := http.Client{Transport: &tr, Timeout: time.Second * 60}

	client := resty.NewWithClient(&cl)
	req := client.R()

	req.SetHeader("api-key", apiKey)

	resp, err := req.Get(fmt.Sprintf("%s/api/v2/auth/apiKey", a.apihubUrl))
	if err != nil || resp.StatusCode() != http.StatusOK {
		if resp.StatusCode() == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	var apiKeyView view.ApihubApiKeyView
	err = json.Unmarshal(resp.Body(), &apiKeyView)
	if err != nil {
		return nil, err
	}

	return &apiKeyView, nil
}

func checkUnauthorized(resp *resty.Response) error {
	if resp != nil && (resp.StatusCode() == http.StatusUnauthorized || resp.StatusCode() == http.StatusForbidden) {
		log.Errorf("Incorrect api key detected!")
		return &exception.CustomError{
			Status:  http.StatusFailedDependency,
			Code:    exception.NoApihubAccess,
			Message: exception.NoApihubAccessMsg,
			Params:  map[string]interface{}{"code": strconv.Itoa(resp.StatusCode())},
		}
	}
	return nil
}

func checkCustomError(resp *resty.Response) error {
	if resp != nil && len(resp.Body()) > 0 {
		var cursomErr exception.CustomError
		jsonErr := json.Unmarshal(resp.Body(), &cursomErr)
		if jsonErr == nil && cursomErr.Code != "" && cursomErr.Message != "" {
			return &cursomErr
		}
	}
	return nil
}

func (a apihubClientImpl) GetRsaPublicKey(ctx context.Context) (*view.PublicKey, error) {
	req := a.makeRequest(ctx)
	resp, err := req.Get(fmt.Sprintf("%s/api/v2/auth/publicKey", a.apihubUrl))
	if err != nil || resp.StatusCode() != http.StatusOK {
		if authErr := checkUnauthorized(resp); authErr != nil {
			return nil, authErr
		}
		// resp could be nil here - in this case the next row will fall to panic()
		if resp.StatusCode() == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	publicKey := view.PublicKey{
		Value: resp.Body(),
	}
	return &publicKey, nil
}

func (a apihubClientImpl) GetVersion(ctx context.Context, id, version string) (*view.VersionContent, error) {

	req := a.makeRequest(ctx)
	resp, err := req.Get(fmt.Sprintf("%s/api/v3/packages/%s/versions/%s", a.apihubUrl, url.PathEscape(id), url.PathEscape(version)))
	if err != nil {
		return nil, fmt.Errorf("failed to get version %s for id %s: %s", version, id, err.Error())
	}
	if resp.StatusCode() != http.StatusOK {
		if resp.StatusCode() == http.StatusNotFound {
			return nil, nil
		}
		if authErr := checkUnauthorized(resp); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("failed to get version %s for id %s: status code %d %v", version, id, resp.StatusCode(), err)
	}
	var pVersion view.VersionContent
	err = json.Unmarshal(resp.Body(), &pVersion)
	if err != nil {
		return nil, err
	}
	return &pVersion, nil
}

func (a apihubClientImpl) GetVersionDocuments(ctx context.Context, packageId, version string) (*view.VersionDocuments, error) {
	req := a.makeRequest(ctx)
	resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s/documents", a.apihubUrl, url.PathEscape(packageId), url.PathEscape(version)))
	if err != nil {
		return nil, fmt.Errorf("failed to get version %s for id %s: %s", version, packageId, err.Error())
	}

	if resp.StatusCode() != http.StatusOK {
		if resp.StatusCode() == http.StatusNotFound {
			return nil, nil
		}
		if authErr := checkUnauthorized(resp); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("failed to get version documents. version - %s for id %s: status code %d %v", version, packageId, resp.StatusCode(), resp.Body())
	}
	var versionDocuments view.VersionDocuments
	err = json.Unmarshal(resp.Body(), &versionDocuments)
	if err != nil {
		return nil, err
	}
	return &versionDocuments, nil
}

func (a apihubClientImpl) GetDocumentRawData(ctx context.Context, packageId, version string, fileSlug string) ([]byte, error) {
	req := a.makeRequest(ctx)
	resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s/files/%s/raw", a.apihubUrl, url.PathEscape(packageId), url.PathEscape(version), url.PathEscape(fileSlug)))
	if err != nil {
		return nil, fmt.Errorf("failed to get document %s for package %s, version %s: %s", fileSlug, packageId, version, err.Error())
	}
	if resp.StatusCode() != http.StatusOK {
		if resp.StatusCode() == http.StatusNotFound {
			return nil, nil
		}
		if authErr := checkUnauthorized(resp); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("failed to get document %s for package %s, version %s: status code %d %v", fileSlug, packageId, version, resp.StatusCode(), resp.Body())
	}

	return resp.Body(), nil
}

func (a apihubClientImpl) CheckAuthToken(ctx context.Context, token string) (bool, error) {
	tr := http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	cl := http.Client{Transport: &tr, Timeout: time.Second * 60}

	client := resty.NewWithClient(&cl)
	req := client.R()
	req.SetContext(ctx)
	req.SetHeader("Cookie", fmt.Sprintf("%s=%s", view.AccessTokenCookieName, token))

	resp, err := req.Get(fmt.Sprintf("%s/api/v1/auth/token", a.apihubUrl))
	if err != nil || resp.StatusCode() != http.StatusOK {
		if authErr := checkUnauthorized(resp); authErr != nil {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

/*
	func (a apihubClientImpl) GetPackageIdByServiceName(ctx secctx.SecurityContext, workspaceId string, serviceName string) (string, string, error) {
		req := a.makeRequest(ctx)

		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages?kind=package&serviceName=%s&parentId=%s&showAllDescendants=true", a.apihubUrl, url.PathEscape(serviceName), workspaceId))
		if err != nil {
			return "", "", err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return "", "", nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return "", "", authErr
			}
			return "", "", fmt.Errorf("failed to get package id by service name -  %s : status code %d %v", serviceName, resp.StatusCode(), err)
		}

		var packages view.SimplePackages

		err = json.Unmarshal(resp.Body(), &packages)
		if err != nil {
			return "", "", err
		}

		if len(packages.Packages) == 0 {
			return "", "", nil
		}

		if len(packages.Packages) != 1 {
			return "", "", fmt.Errorf("unable to get package by id: unexpected number of packages returned %d", len(packages.Packages))
		}
		return packages.Packages[0].Id, packages.Packages[0].Name, nil
	}

	func (a apihubClientImpl) GetPackageByServiceName(ctx secctx.SecurityContext, workspaceId string, serviceName string) (*view.SimplePackage, error) {
		req := a.makeRequest(ctx)

		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages?kind=package&serviceName=%s&parentId=%s&showAllDescendants=true", a.apihubUrl, url.PathEscape(serviceName), workspaceId))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get package id by service name -  %s : status code %d %v", serviceName, resp.StatusCode(), err)
		}

		var packages view.SimplePackages

		err = json.Unmarshal(resp.Body(), &packages)
		if err != nil {
			return nil, err
		}

		if len(packages.Packages) == 0 {
			return nil, nil
		}

		if len(packages.Packages) != 1 {
			return nil, fmt.Errorf("unable to get package by id: unexpected number of packages returned %d", len(packages.Packages))
		}
		pkg := packages.Packages[0]
		return &pkg, nil
	}

	func (a apihubClientImpl) GetPackages(ctx secctx.SecurityContext, searchReq view.PackagesSearchReq) (*view.SimplePackages, error) {
		req := a.makeRequest(ctx)

		base, err := url.Parse(fmt.Sprintf("%s/api/v2/packages", a.apihubUrl))
		if err != nil {
			return nil, err
		}
		if searchReq.Limit == 0 {
			searchReq.Limit = 100
		}
		params := url.Values{}
		if searchReq.TextFilter != "" {
			params.Add("textFilter", searchReq.TextFilter)
		}
		if searchReq.ServiceName != "" {
			params.Add("serviceName", searchReq.ServiceName)
		}
		if searchReq.ParentId != "" {
			params.Add("parentId", searchReq.ParentId)
		}
		if searchReq.ShowAllDescendants {
			params.Add("showAllDescendants", strconv.FormatBool(searchReq.ShowAllDescendants))
		}
		if searchReq.ShowParents {
			params.Add("showParents", strconv.FormatBool(searchReq.ShowParents))
		}
		if searchReq.Kind != "" {
			params.Add("kind", searchReq.Kind)
		}
		base.RawQuery = params.Encode()

		var uri string
		if len(params) > 0 {
			uri = fmt.Sprintf("%s&limit=%d&page=%d", base.String(), searchReq.Limit, searchReq.Page)
		} else {
			uri = fmt.Sprintf("%s?limit=%d&page=%d", base.String(), searchReq.Limit, searchReq.Page)
		}
		resp, err := req.Get(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to get package by serarchReq : %v. Response status code %d %v", searchReq, resp.StatusCode(), err)
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get packages by request -  %v : status code %d %v", searchReq, resp.StatusCode(), err)
		}

		var packages view.SimplePackages

		err = json.Unmarshal(resp.Body(), &packages)
		if err != nil {
			return nil, err
		}
		if len(packages.Packages) == 0 {
			return nil, nil
		}
		return &packages, nil
	}

	func (a apihubClientImpl) CreatePackage(ctx secctx.SecurityContext, pkg view.PackageCreateRequest) (string, error) {
		req := a.makeRequest(ctx)
		req.SetBody(pkg)

		resp, err := req.Post(fmt.Sprintf("%s/api/v2/packages", a.apihubUrl))
		if err != nil {
			return "", err
		}
		if resp.StatusCode() != http.StatusCreated {
			if authErr := checkUnauthorized(resp); authErr != nil {
				return "", authErr
			}
			return "", fmt.Errorf("failed to create package by request -  %v : status code %d %v", pkg, resp.StatusCode(), err)
		}
		var res view.PackageResponse
		err = json.Unmarshal(resp.Body(), &res)
		if err != nil {
			return "", err
		}
		return res.Id, nil
	}

	func (a apihubClientImpl) PatchPackage(ctx secctx.SecurityContext, packageId string, patchReq view.PackagePatchReq) (*view.SimplePackage, error) {
		req := a.makeRequest(ctx)
		req.SetBody(patchReq)
		resp, err := req.Patch(fmt.Sprintf("%s/api/v2/packages/%s", a.apihubUrl, packageId))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to patch package by request -  %v : status code %d %v", patchReq, resp.StatusCode(), err)
		}
		var res view.SimplePackage
		err = json.Unmarshal(resp.Body(), &res)
		if err != nil {
			return nil, err
		}
		return &res, nil
	}

	func (a apihubClientImpl) GetVersions(ctx secctx.SecurityContext, packageId string, searchReq view.VersionSearchRequest) (*view.PublishedVersionsView, error) {
		req := a.makeRequest(ctx)
		base, err := url.Parse(fmt.Sprintf("%s/api/v2/packages/%s/versions", a.apihubUrl, url.PathEscape(packageId)))
		if err != nil {
			return nil, err
		}

		if searchReq.Limit == 0 {
			searchReq.Limit = 100
		}
		params := url.Values{}
		if searchReq.Status != "" {
			params.Add("status", searchReq.Status)
		}
		if searchReq.VersionLabel != "" {
			params.Add("versionLabel", searchReq.VersionLabel)
		}
		if searchReq.TextFilter != "" {
			params.Add("textFilter", searchReq.TextFilter)
		}
		if searchReq.CheckRevisions {
			params.Add("checkRevisions", strconv.FormatBool(searchReq.CheckRevisions))
		}
		if searchReq.SortBy != "" {
			params.Add("sortBy", searchReq.SortBy)
		} else {
			params.Add("sortBy", view.VersionSortByCreatedAt)
		}
		if searchReq.SortOrder != "" {
			params.Add("sortOrder", searchReq.SortOrder)
		} else {
			params.Add("sortOrder", view.VersionSortOrderDesc)
		}
		base.RawQuery = params.Encode()
		var uri string
		if len(params) > 0 {
			uri = fmt.Sprintf("%s&limit=%d&page=%d", base.String(), searchReq.Limit, searchReq.Page)
		} else {
			uri = fmt.Sprintf("%s?limit=%d&page=%d", base.String(), searchReq.Limit, searchReq.Page)
		}
		resp, err := req.Get(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to get versions for package with id %s. Error - %s", packageId, err.Error())
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get versions for package with id %s. Response status code %d %v", packageId, resp.StatusCode(), err)
		}
		var pVersions view.PublishedVersionsView
		err = json.Unmarshal(resp.Body(), &pVersions)
		if err != nil {
			return nil, err
		}
		return &pVersions, nil
	}

	func (a apihubClientImpl) GetVersionInfo(ctx secctx.SecurityContext, id, version string) (*view.VersionContent, error) {
		req := a.makeRequest(ctx)
		req.QueryParam.Add("includeSummary", "true") //required to fill ChangeSummary and ApiTypes fields
		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s", a.apihubUrl, url.PathEscape(id), url.PathEscape(version)))
		if err != nil {
			return nil, fmt.Errorf("failed to get version %s for id %s: %s", version, id, err.Error())
		}

		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get version %s for id %s: status code %d %v", version, id, resp.StatusCode(), err)
		}
		var responseVersion view.VersionContentApihubResponse
		err = json.Unmarshal(resp.Body(), &responseVersion)
		if err != nil {
			return nil, err
		}
		versionContent := view.VersionContent{
			PublishedAt:              responseVersion.PublishedAt,
			PublishedBy:              responseVersion.PublishedBy,
			PreviousVersion:          responseVersion.PreviousVersion,
			PreviousVersionPackageId: responseVersion.PreviousVersionPackageId,
			VersionLabels:            responseVersion.VersionLabels,
			ApiTypes:                 make([]string, 0),
			PackageId:                responseVersion.PackageId,
			Version:                  responseVersion.Version,
			NotLatestRevision:        responseVersion.NotLatestRevision,
			Status:                   responseVersion.Status,
			// OperationTypes:           responseVersion.OperationTypes,
		}
		for _, operationType := range responseVersion.OperationTypes {
			versionContent.ApiTypes = append(versionContent.ApiTypes, operationType.ApiType)
			if operationType.ChangesSummary != nil {
				changeSummary := *operationType.ChangesSummary
				if versionContent.ChangeSummary == nil {
					versionContent.ChangeSummary = &changeSummary
				} else {
					operationType.ChangesSummary.Breaking += changeSummary.Breaking
					operationType.ChangesSummary.SemiBreaking += changeSummary.SemiBreaking
					operationType.ChangesSummary.Deprecated += changeSummary.Deprecated
					operationType.ChangesSummary.NonBreaking += changeSummary.NonBreaking
					operationType.ChangesSummary.Annotation += changeSummary.Annotation
					operationType.ChangesSummary.Unclassified += changeSummary.Unclassified
				}
			}
		}
		return &versionContent, nil
	}

	func (a apihubClientImpl) GetVersion(ctx secctx.SecurityContext, id, version string) (*view.VersionContent, error) {
		req := a.makeRequest(ctx)
		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s?includeSummary=true&includeOperations=true", a.apihubUrl, url.PathEscape(id), url.PathEscape(version)))
		if err != nil {
			return nil, fmt.Errorf("failed to get version %s for id %s: %s", version, id, err.Error())
		}

		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get version %s for id %s: status code %d %v", version, id, resp.StatusCode(), err)
		}
		var responseVersion view.VersionContentApihubResponse
		err = json.Unmarshal(resp.Body(), &responseVersion)
		if err != nil {
			return nil, err
		}
		versionContent := view.VersionContent{
			PublishedAt:              responseVersion.PublishedAt,
			PublishedBy:              responseVersion.PublishedBy,
			PreviousVersion:          responseVersion.PreviousVersion,
			PreviousVersionPackageId: responseVersion.PreviousVersionPackageId,
			VersionLabels:            responseVersion.VersionLabels,
			ApiTypes:                 make([]string, 0),
			PackageId:                responseVersion.PackageId,
			Version:                  responseVersion.Version,
			NotLatestRevision:        responseVersion.NotLatestRevision,
			Status:                   responseVersion.Status,
			OperationTypes:           responseVersion.OperationTypes,
		}
		for _, operationType := range responseVersion.OperationTypes {
			versionContent.ApiTypes = append(versionContent.ApiTypes, operationType.ApiType)
			if operationType.ChangesSummary != nil {
				changeSummary := *operationType.ChangesSummary
				if versionContent.ChangeSummary == nil {
					versionContent.ChangeSummary = &changeSummary
				} else {
					versionContent.ChangeSummary.Breaking += changeSummary.Breaking
					versionContent.ChangeSummary.SemiBreaking += changeSummary.SemiBreaking
					versionContent.ChangeSummary.Deprecated += changeSummary.Deprecated
					versionContent.ChangeSummary.NonBreaking += changeSummary.NonBreaking
					versionContent.ChangeSummary.Annotation += changeSummary.Annotation
					versionContent.ChangeSummary.Unclassified += changeSummary.Unclassified
				}
			}
		}
		return &versionContent, nil
	}

	func (a apihubClientImpl) GetVersionReferences(ctx secctx.SecurityContext, id, version string) (*view.VersionReferences, error) {
		req := a.makeRequest(ctx)
		resp, err := req.Get(fmt.Sprintf("%s/api/v3/packages/%s/versions/%s/references", a.apihubUrl, url.PathEscape(id), url.PathEscape(version)))
		if err != nil {
			return nil, fmt.Errorf("failed to get version %s for id %s: %s", version, id, err.Error())
		}

		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get version references. version - %s for id %s: status code %d %v", version, id, resp.StatusCode(), err)
		}
		var versionReferences view.VersionReferences
		err = json.Unmarshal(resp.Body(), &versionReferences)
		if err != nil {
			return nil, err
		}
		return &versionReferences, nil
	}

	func (a apihubClientImpl) GetVersionChanges(ctx secctx.SecurityContext, packageId string, version string, prevVersionPackageId string, prevVersion string, apiType string, limit int, page int) (*view.VersionChangesView, error) {
		req := a.makeRequest(ctx)
		if prevVersionPackageId == "" {
			prevVersionPackageId = packageId
		}
		req.QueryParam.Add("limit", strconv.Itoa(limit))
		req.QueryParam.Add("page", strconv.Itoa(page))
		req.QueryParam.Add("previousVersionPackageId", prevVersionPackageId)
		req.QueryParam.Add("previousVersion", prevVersion)
		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s/%s/changes", a.apihubUrl, url.PathEscape(packageId), url.PathEscape(version), apiType))
		if err != nil {
			return nil, fmt.Errorf("failed to get version - %s for package id %s: %s", version, packageId, err.Error())
		}

		if resp.StatusCode() != http.StatusOK {
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			if resp.StatusCode() == http.StatusNotFound {
				log.Infof("Missing changes detected for packageId = %s, version = %s, prev version = %s. Trying to build it.", packageId, version, prevVersion)
				timeout := time.Second * 120
				for {
					if timeout <= 0 {
						log.Warnf("VersionChanges timed out for package %s version %s!", packageId, version)
						return nil, fmt.Errorf("failed to get version changes due to timeout. version %s for package id %s", version, packageId)
					}
					compReqBody := view.CompareVersionsReq{
						PackageId:                packageId,
						Version:                  version,
						PreviousVersion:          prevVersion,
						PreviousVersionPackageId: prevVersionPackageId,
					}

					compReq := a.makeRequest(ctx)
					compReq.SetBody(compReqBody)
					compResp, err := compReq.Post(fmt.Sprintf("%s/api/v2/compare", a.apihubUrl))
					if err != nil {
						if authErr := checkUnauthorized(compResp); authErr != nil {
							return nil, authErr
						}
						return nil, fmt.Errorf("failed to start %s version comparison for package id %s: %s", version, packageId, err.Error())
					}
					if compResp.StatusCode() == http.StatusCreated || compResp.StatusCode() == http.StatusAccepted {
						timeout -= time.Second * 1
						time.Sleep(time.Second * 1)
						continue
					}
					if compResp.StatusCode() == http.StatusOK {
						resp, err = req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s/%s/changes", a.apihubUrl, url.PathEscape(packageId), url.PathEscape(version), apiType))
						if err != nil {
							return nil, fmt.Errorf("failed to get version %s changes after generate for package id %s: %s", version, packageId, err.Error())
						}

						if resp.StatusCode() == http.StatusOK {
							break
						} else {
							if authErr := checkUnauthorized(resp); authErr != nil {
								return nil, authErr
							}
							return nil, fmt.Errorf("failed to get version changes after generate. version - %s for package id %s: status code %d %v", version, packageId, resp.StatusCode(), err)
						}
					} else {
						if authErr := checkUnauthorized(resp); authErr != nil {
							return nil, authErr
						}
						return nil, fmt.Errorf("failed to start %s version comparison for package id %s: status code %d %v", version, packageId, resp.StatusCode(), err)
					}
				}
			} else {
				return nil, fmt.Errorf("failed to get version changes. version - %s for package id %s: status code %d %v", version, packageId, resp.StatusCode(), err)
			}
		}
		var pVersion view.VersionChangesView
		err = json.Unmarshal(resp.Body(), &pVersion)
		if err != nil {
			return nil, err
		}
		return &pVersion, nil
	}

	func (a apihubClientImpl) GetVersionSources(ctx secctx.SecurityContext, packageId, version string) (*view.PublishedVersionSourceDataConfig, error) {
		req := a.makeRequest(ctx)
		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s/sourceData", a.apihubUrl, url.PathEscape(packageId), url.PathEscape(version)))
		if err != nil {
			return nil, fmt.Errorf("failed to get version sources. Error - %s", err.Error())
		}

		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get version sources: status code %d %v", resp.StatusCode(), err)
		}

		var publishedVersionSourceDataConfig view.PublishedVersionSourceDataConfig
		err = json.Unmarshal(resp.Body(), &publishedVersionSourceDataConfig)
		if err != nil {
			return nil, err
		}
		return &publishedVersionSourceDataConfig, nil
	}

	func (a apihubClientImpl) GetVersionRestOperationsWithData(ctx secctx.SecurityContext, packageId string, version string, limit int, page int) (*view.RestOperations, error) {
		req := a.makeRequest(ctx)
		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s/rest/operations?includeData=true&limit=%d&page=%d", a.apihubUrl, url.PathEscape(packageId), url.PathEscape(version), limit, page))
		if err != nil {
			return nil, fmt.Errorf("failed to get version rest operations. Error - %s", err.Error())
		}

		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get version rest operations: status code %d %v", resp.StatusCode(), err)
		}

		var restOperations view.RestOperations
		err = json.Unmarshal(resp.Body(), &restOperations)
		if err != nil {
			return nil, err
		}
		return &restOperations, nil
	}

	func (a apihubClientImpl) GetOperationsList(ctx secctx.SecurityContext, packageId string, version string, apiType string, operationListReq view.OperationListRequest) (*view.CommonOperations, error) {
		req := a.makeRequest(ctx)
		req.QueryParam.Add("page", strconv.Itoa(operationListReq.Page))
		req.QueryParam.Add("limit", strconv.Itoa(operationListReq.Limit))
		req.QueryParam.Add("deprecated", operationListReq.Deprecated)
		req.QueryParam.Add("kind", operationListReq.Kind)
		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s/%s/operations", a.apihubUrl, url.PathEscape(packageId), url.PathEscape(version), apiType))
		if err != nil {
			return nil, fmt.Errorf("failed to get version rest operations. Error - %s", err.Error())
		}

		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get version rest operations: status code %d %v", resp.StatusCode(), err)
		}

		var operations view.CommonOperations
		err = json.Unmarshal(resp.Body(), &operations)
		if err != nil {
			return nil, err
		}
		return &operations, nil
	}

	func (a apihubClientImpl) GetApihubUrl() string {
		return a.apihubUrl
	}

	func (a apihubClientImpl) GetPackageById(ctx secctx.SecurityContext, id string) (*view.SimplePackage, error) {
		req := a.makeRequest(ctx)

		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s", a.apihubUrl, url.PathEscape(id)))

		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get package by id -  %s : status code %d %v", id, resp.StatusCode(), err)
		}

		var pkg view.SimplePackage

		err = json.Unmarshal(resp.Body(), &pkg)
		if err != nil {
			return nil, err
		}
		return &pkg, nil
	}

	func (a apihubClientImpl) GetUserPackagesPromoteStatuses(ctx secctx.SecurityContext, packagesReq view.PackagesReq) (view.AvailablePackagePromoteStatuses, error) {
		req := a.makeRequest(ctx)
		req.SetBody(packagesReq)

		resp, err := req.Post(fmt.Sprintf("%s/api/v2/users/%s/availablePackagePromoteStatuses", a.apihubUrl, url.QueryEscape(ctx.GetUserId())))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get user packages promote statuses by request -  %v : status code %d %v", packagesReq, resp.StatusCode(), err)
		}

		var result view.AvailablePackagePromoteStatuses
		err = json.Unmarshal(resp.Body(), &result)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	func (a apihubClientImpl) Publish(ctx secctx.SecurityContext, config view.BuildConfig, src []byte, clientBuild bool, builderId string, saveSources bool, dependencies []string) (string, error) {
		req := a.makeRequest(ctx)

		confBytes, err := json.Marshal(config)
		if err != nil {
			return "", err
		}

		depBytes, err := json.Marshal(dependencies)
		if err != nil {
			return "", err
		}

		var data []*resty.MultipartField
		data = append(data, &resty.MultipartField{
			Param:  "config",
			Reader: bytes.NewReader(confBytes),
		})
		data = append(data, &resty.MultipartField{
			Param:  "clientBuild",
			Reader: strings.NewReader(strconv.FormatBool(clientBuild)),
		})
		data = append(data, &resty.MultipartField{
			Param:  "saveSources",
			Reader: strings.NewReader(strconv.FormatBool(saveSources)),
		})
		data = append(data, &resty.MultipartField{
			Param:  "dependencies",
			Reader: bytes.NewReader(depBytes),
		})
		if builderId != "" {
			data = append(data, &resty.MultipartField{
				Param:  "builderId",
				Reader: strings.NewReader(builderId),
			})
		}

		if src != nil {
			req.SetFileReader("sources", "sources.zip", bytes.NewReader(src))
		}

		req.SetMultipartFields(data...)

		resp, err := req.Post(fmt.Sprintf("%s/api/v2/packages/%s/publish", a.apihubUrl, url.PathEscape(config.PackageId)))
		if err != nil {
			return "", fmt.Errorf("failed to build and publish package %s: %s", config.PackageId, err.Error())
		}
		if !(resp.StatusCode() == http.StatusAccepted || resp.StatusCode() == http.StatusNoContent) {
			if authErr := checkUnauthorized(resp); authErr != nil {
				return "", authErr
			}
			return "", fmt.Errorf("failed to build and publish package %s: status code = %d, body = %s", config.PackageId, resp.StatusCode(), string(resp.Body()))
		}
		var publishResponse view.PublishId
		if err = json.Unmarshal(resp.Body(), &publishResponse); err != nil {
			return "", err
		}
		return publishResponse.PublishId, nil
	}

	func (a apihubClientImpl) GetPublishStatus(ctx secctx.SecurityContext, packageId, publishId string) (view.StatusEnum, string, error) {
		req := a.makeRequest(ctx)
		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/publish/%s/status", a.apihubUrl, url.PathEscape(packageId), url.PathEscape(publishId)))
		if err != nil {
			return "", "", err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return "", "", nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return "", "", authErr
			}
			return "", "", fmt.Errorf("failed to get publish status. packageId -  %s, publishId - %s : status code %d %v", packageId, publishId, resp.StatusCode(), err)
		}
		var psr view.PublishStatusResponse
		err = json.Unmarshal(resp.Body(), &psr)
		if err != nil {
			return "", "", err
		}
		status, err := view.BuildStatusFromString(psr.Status)
		if err != nil {
			return "", "", err
		}
		return status, psr.Message, nil
	}

	func (a apihubClientImpl) GetPublishStatuses(ctx secctx.SecurityContext, packageId string, publishIds []string) ([]view.PublishStatusResponse, error) {
		req := a.makeRequest(ctx)
		req.SetBody(map[string]interface{}{
			"publishIds": publishIds,
		})
		resp, err := req.Post(fmt.Sprintf("%s/api/v2/packages/%s/publish/statuses", a.apihubUrl, packageId))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get build statuses for %v: status code %d %v", publishIds, resp.StatusCode(), err)
		}
		var res []view.PublishStatusResponse
		err = json.Unmarshal(resp.Body(), &res)
		if err != nil {
			return nil, err
		}
		return res, nil
	}

	func (a apihubClientImpl) GetAgent(ctx secctx.SecurityContext, agentId string) (*view.AgentInstance, error) {
		req := a.makeRequest(ctx)

		resp, err := req.Get(fmt.Sprintf("%s/api/v2/agents/%s", a.apihubUrl, agentId))

		if err != nil {
			return nil, err
		}

		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get agent by id - %s : status code %d %v", agentId, resp.StatusCode(), err)
		}
		var agentInstance view.AgentInstance

		err = json.Unmarshal(resp.Body(), &agentInstance)
		if err != nil {
			return nil, err
		}

		if agentInstance.Status == view.AgentStatusInactive {
			return nil, &exception.CustomError{
				Status:  http.StatusFailedDependency,
				Code:    exception.InactiveAgent,
				Message: exception.InactiveAgentMsg,
				Params:  map[string]interface{}{"id": agentId},
			}
		}
		if agentInstance.AgentVersion == "" {
			return nil, &exception.CustomError{
				Status:  http.StatusFailedDependency,
				Code:    exception.IncompatibleAgentVersion,
				Message: exception.IncompatibleAgentVersionMsg,
			}
		}
		if agentInstance.CompatibilityError != nil && agentInstance.CompatibilityError.Severity == view.SeverityError {
			return nil, &exception.CustomError{
				Status:  http.StatusFailedDependency,
				Code:    exception.IncompatibleAgentVersion,
				Message: agentInstance.CompatibilityError.Message,
			}
		}

		return &agentInstance, nil
	}

	func (a apihubClientImpl) GetAgents(ctx secctx.SecurityContext) ([]view.AgentInstance, error) {
		req := a.makeRequest(ctx)

		resp, err := req.Get(fmt.Sprintf("%s/api/v2/agents", a.apihubUrl))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get agent agents : status code %d %v", resp.StatusCode(), err)
		}
		var agentInstances []view.AgentInstance

		err = json.Unmarshal(resp.Body(), &agentInstances)
		if err != nil {
			return nil, err
		}
		return agentInstances, nil
	}

	func (a apihubClientImpl) GetSystemInfo(ctx secctx.SecurityContext) (*view.ApihubSystemInfo, error) {
		req := a.makeRequest(ctx)
		resp, err := req.Get(fmt.Sprintf("%s/api/v1/system/info", a.apihubUrl))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get apihub system info : status code %d %v", resp.StatusCode(), err)
		}
		var info view.ApihubSystemInfo
		err = json.Unmarshal(resp.Body(), &info)
		if err != nil {
			return nil, err
		}
		return &info, nil
	}

	func (a apihubClientImpl) GetUserById(ctx secctx.SecurityContext, userId string) (*view.User, error) {
		req := a.makeRequest(ctx)
		resp, err := req.Get(fmt.Sprintf("%s/api/v2/users/%s", a.apihubUrl, userId))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get user info: status code %d %v", resp.StatusCode(), err)
		}
		var user view.User
		err = json.Unmarshal(resp.Body(), &user)
		if err != nil {
			return nil, err
		}
		return &user, nil
	}

	func (a apihubClientImpl) GetApiKeyById(ctx secctx.SecurityContext, apiKeyId string) (*view.ApihubApiKeyView, error) {
		req := a.makeRequest(ctx)
		resp, err := req.Get(fmt.Sprintf("%s/api/v1/auth/apiKey/%s", a.apihubUrl, apiKeyId))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get api-key info: status code %d %v", resp.StatusCode(), err)
		}
		var apiKey view.ApihubApiKeyView
		err = json.Unmarshal(resp.Body(), &apiKey)
		if err != nil {
			return nil, err
		}
		return &apiKey, nil
	}

	func (a apihubClientImpl) GetVersionRevisions(ctx secctx.SecurityContext, id, version string, limit int, page int) (*view.VersionRevisionsView, error) {
		req := a.makeRequest(ctx)
		req.SetQueryParams(map[string]string{
			"limit": strconv.Itoa(limit),
			"page":  strconv.Itoa(page),
		})
		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s/revisions", a.apihubUrl, id, version))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get version revisions: status code %d %v", resp.StatusCode(), err)
		}
		var revisions view.VersionRevisionsView
		err = json.Unmarshal(resp.Body(), &revisions)
		if err != nil {
			return nil, err
		}
		return &revisions, nil
	}

	func (a apihubClientImpl) GetPackageActivityHistory(ctx secctx.SecurityContext, packageId string, parameters view.GetPackageActivityEventsReq) (*view.PackageActivityEvents, error) {
		req := a.makeRequest(ctx)
		if parameters.IncludeRefs {
			req.SetQueryParam("includeRefs", strconv.FormatBool(parameters.IncludeRefs))
		}
		if parameters.Limit > 0 {
			req.SetQueryParam("limit", strconv.Itoa(parameters.Limit))
		}
		if parameters.Page >= 0 {
			req.SetQueryParam("page", strconv.Itoa(parameters.Page))
		}
		if parameters.TextFilter != "" {
			req.SetQueryParam("textFilter", parameters.TextFilter)
		}
		if len(parameters.Types) > 0 {
			req.SetQueryParam("types", strings.Join(parameters.Types, ","))
		}

		resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/activity", a.apihubUrl, packageId))
		if resp.StatusCode() != http.StatusOK {
			if resp.StatusCode() == http.StatusNotFound {
				return nil, nil
			}
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get version revisions: status code %d %v", resp.StatusCode(), err)
		}
		var events view.PackageActivityEvents
		err = json.Unmarshal(resp.Body(), &events)
		if err != nil {
			return nil, err
		}
		return &events, nil
	}

	func (a apihubClientImpl) ListCompletedActivities(offset int, limit int) ([]view.TransitionStatus, error) {
		req := a.makeRequest(secctx.CreateSystemContext())
		req.QueryParam.Add("offset", strconv.Itoa(offset))
		req.QueryParam.Add("limit", strconv.Itoa(limit))

		resp, err := req.Get(fmt.Sprintf("%s/api/v2/admin/transition/activity", a.apihubUrl))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to send transition activity request with error code %d", resp.StatusCode())
		}
		var transitionActivities []view.TransitionStatus
		err = json.Unmarshal(resp.Body(), &transitionActivities)
		if err != nil {
			return nil, err
		}
		return transitionActivities, nil
	}

	func (a apihubClientImpl) ListPackageTransitions() ([]view.PackageTransition, error) {
		req := a.makeRequest(secctx.CreateSystemContext())

		resp, err := req.Get(fmt.Sprintf("%s/api/v2/admin/transition", a.apihubUrl))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to send package transitions request with error code %d", resp.StatusCode())
		}
		var transitions []view.PackageTransition
		err = json.Unmarshal(resp.Body(), &transitions)
		if err != nil {
			return nil, err
		}
		return transitions, nil
	}

	func (a apihubClientImpl) GetPublishedVersionsHistory(status string, publishedAfter *time.Time, publishedBefore *time.Time, limit int, page int) ([]view.PublishedVersionHistoryView, error) {
		req := a.makeRequest(secctx.CreateSystemContext())
		req.QueryParam.Add("limit", strconv.Itoa(limit))
		req.QueryParam.Add("page", strconv.Itoa(page))
		if status != "" {
			req.QueryParam.Add("status", status)
		}
		if publishedAfter != nil {
			req.QueryParam.Add("publishedAfter", publishedAfter.Format(time.RFC3339))
		}
		if publishedBefore != nil {
			req.QueryParam.Add("publishedBefore", publishedBefore.Format(time.RFC3339))
		}
		resp, err := req.Get(fmt.Sprintf("%s/api/v1/publishHistory", a.apihubUrl))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			if authErr := checkUnauthorized(resp); authErr != nil {
				return nil, authErr
			}
			return nil, fmt.Errorf("failed to get published versions history: status code %d", resp.StatusCode())
		}
		var history []view.PublishedVersionHistoryView
		err = json.Unmarshal(resp.Body(), &history)
		if err != nil {
			return nil, err
		}
		return history, nil
	}

	func (a apihubClientImpl) DeleteVersionsRecursively(packageId string, parameters view.DeleteVersionsRecursivelyReq) (string, error) {
		req := a.makeRequest(secctx.CreateSystemContext())

		req.SetBody(parameters)
		resp, err := req.Post(fmt.Sprintf("%s/api/v2/packages/%s/versions/recursiveDelete", a.apihubUrl, packageId))
		if err != nil {
			return "", err
		}
		if resp.StatusCode() != http.StatusOK {
			if authErr := checkUnauthorized(resp); authErr != nil {
				return "", authErr
			}
			if resp.StatusCode() == http.StatusNotFound {
				return "", nil
			}
			return "", fmt.Errorf("failed to send delete versions by retention request with error code %d", resp.StatusCode())
		}
		var response view.DeleteVersionsRecursiveResponse
		err = json.Unmarshal(resp.Body(), &response)
		if err != nil {
			return "", err
		}
		return response.JobId, nil
	}
*/
func (a apihubClientImpl) makeRequest(ctx context.Context) *resty.Request {
	req := a.client.R()
	req.SetContext(ctx)

	if secctx.IsSystem(ctx) {
		req.SetHeader("api-key", a.accessToken)
	} else {
		if secctx.GetUserToken(ctx) != "" {
			req.SetHeader("Authorization", fmt.Sprintf("Bearer %s", secctx.GetUserToken(ctx)))
		} else if secctx.GetApiKey(ctx) != "" {
			req.SetHeader("api-key", secctx.GetApiKey(ctx))
		}
	}
	return req
}
