package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Netcracker/qubership-api-linter-service/exception"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/view"

	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/resty.v1"
)

type ApihubClient interface {
	GetRsaPublicKey(ctx context.Context) (*view.PublicKey, error)
	GetApiKeyByKey(apiKey string) (*view.ApihubApiKeyView, error)

	GetPackageById(ctx context.Context, id string) (*view.SimplePackage, error)
	GetVersion(ctx context.Context, id, version string) (*view.VersionContent, error)

	GetVersionDocuments(ctx context.Context, packageId, version string) (*view.VersionDocuments, error)
	GetDocumentRawData(ctx context.Context, packageId, version string, fileSlug string) ([]byte, error)

	CheckAuthToken(ctx context.Context, token string) (bool, error)
	GetUserByPAT(ctx context.Context, token string) (*view.User, error)
	GetPatByPAT(ctx context.Context, token string) (*view.PersonalAccessTokenExtAuthView, error)

	GetAvailableRoles(ctx context.Context, packageId string) (*view.PackageRoles, error)

	GetSystemInfo(ctx context.Context) (*view.ApihubSystemInfo, error)

	GetVersionSources(ctx context.Context, packageId, version string) (*view.PublishedVersionSourceDataConfig, error)

	PublishVersion(ctx context.Context, config view.BuildConfig, src []byte) (string, error)
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

func (a apihubClientImpl) GetPackageById(ctx context.Context, id string) (*view.SimplePackage, error) {
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

func (a apihubClientImpl) GetUserByPAT(ctx context.Context, token string) (*view.User, error) {
	tr := http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	cl := http.Client{Transport: &tr, Timeout: time.Second * 60}

	client := resty.NewWithClient(&cl)
	req := client.R()
	req.SetContext(ctx)
	req.SetHeader("X-Personal-Access-Token", token)

	resp, err := req.Get(fmt.Sprintf("%s/api/v1/user", a.apihubUrl))
	if err != nil || resp.StatusCode() != http.StatusOK {
		if authErr := checkUnauthorized(resp); authErr != nil {
			return nil, nil
		}
		return nil, err
	}

	var user view.User
	err = json.Unmarshal(resp.Body(), &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (a apihubClientImpl) GetPatByPAT(ctx context.Context, token string) (*view.PersonalAccessTokenExtAuthView, error) {
	tr := http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	cl := http.Client{Transport: &tr, Timeout: time.Second * 60}

	client := resty.NewWithClient(&cl)
	req := client.R()
	req.SetContext(ctx)
	req.SetHeader("X-Personal-Access-Token", token)

	resp, err := req.Get(fmt.Sprintf("%s/api/v2/auth/pat", a.apihubUrl))
	if err != nil || resp.StatusCode() != http.StatusOK {
		if authErr := checkUnauthorized(resp); authErr != nil {
			return nil, nil
		}
		return nil, err
	}

	var pat view.PersonalAccessTokenExtAuthView
	err = json.Unmarshal(resp.Body(), &pat)
	if err != nil {
		return nil, err
	}

	return &pat, nil
}

func (a apihubClientImpl) GetAvailableRoles(ctx context.Context, packageId string) (*view.PackageRoles, error) {
	req := a.makeRequest(ctx)

	resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/availableRoles", a.apihubUrl, packageId))
	if err != nil || resp.StatusCode() != http.StatusOK {
		if authErr := checkUnauthorized(resp); authErr != nil {
			return nil, nil
		}
		return nil, err
	}

	var roles view.PackageRoles
	err = json.Unmarshal(resp.Body(), &roles)
	if err != nil {
		return nil, err
	}

	return &roles, nil
}

func (a apihubClientImpl) GetSystemInfo(ctx context.Context) (*view.ApihubSystemInfo, error) {
	req := a.makeRequest(ctx)
	resp, err := req.Get(fmt.Sprintf("%s/api/v1/system/info", a.apihubUrl))
	if err != nil {
		return nil, fmt.Errorf("failed to get APIHUB system info: %s", err.Error())
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get APIHUB system info: status code %d", resp.StatusCode())
	}
	var config view.ApihubSystemInfo
	err = json.Unmarshal(resp.Body(), &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (a apihubClientImpl) GetVersionSources(ctx context.Context, packageId, version string) (*view.PublishedVersionSourceDataConfig, error) {
	req := a.makeRequest(ctx)
	resp, err := req.Get(fmt.Sprintf("%s/api/v2/packages/%s/versions/%s/sourceData", a.apihubUrl, url.PathEscape(packageId), url.PathEscape(version)))
	if err != nil {
		return nil, fmt.Errorf("failed to get version sources: %s", err.Error())
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get version sources: status code %d", resp.StatusCode())
	}
	var src view.PublishedVersionSourceDataConfig
	err = json.Unmarshal(resp.Body(), &src)
	if err != nil {
		return nil, err
	}
	return &src, nil
}

func (a apihubClientImpl) PublishVersion(ctx context.Context, config view.BuildConfig, src []byte) (string, error) {
	req := a.makeRequest(ctx)

	confBytes, err := json.Marshal(config)
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
		Reader: strings.NewReader(strconv.FormatBool(false)),
	})

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
	var publishResponse view.PublishResponse
	if err = json.Unmarshal(resp.Body(), &publishResponse); err != nil {
		return "", err
	}
	return publishResponse.PublishId, nil
}

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
		} else if secctx.GetPersonalAccessToken(ctx) != "" {
			req.SetHeader("X-Personal-Access-Token", secctx.GetPersonalAccessToken(ctx))
		}
	}
	return req
}
