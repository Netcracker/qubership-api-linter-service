package secctx

import (
	"github.com/shaj13/go-guardian/v2/auth"
	"net/http"
	"strings"
)

type SecurityContext interface {
	GetUserId() string
	GetUserToken() string
	GetApiKey() string
	IsSystem() bool
}

func Create(r *http.Request) SecurityContext {
	user := auth.User(r)
	userId := user.GetID()
	token := getAuthorizationToken(r)
	if token != "" {
		return &securityContextImpl{
			userId:   userId,
			token:    token,
			apiKey:   "",
			isSystem: false,
		}
	} else {
		return &securityContextImpl{
			userId:   userId,
			token:    "",
			apiKey:   getApihubApiKey(r),
			isSystem: false,
		}
	}
}

func CreateSystemContext() SecurityContext {
	return &securityContextImpl{isSystem: true}
}

type securityContextImpl struct {
	userId   string
	token    string
	apiKey   string
	isSystem bool
}

func getAuthorizationToken(r *http.Request) string {
	authorizationHeaderValue := r.Header.Get("authorization")
	return strings.ReplaceAll(authorizationHeaderValue, "Bearer ", "")
}

func getApihubApiKey(r *http.Request) string {
	return r.Header.Get("api-key")
}

func (ctx securityContextImpl) GetUserId() string {
	return ctx.userId
}
func (ctx securityContextImpl) GetUserToken() string {
	return ctx.token
}
func (ctx securityContextImpl) GetApiKey() string {
	return ctx.apiKey
}
func (ctx securityContextImpl) IsSystem() bool { return ctx.isSystem }
