package secctx

import (
	"context"
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

// deprecated
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

func MakeUserContext(r *http.Request) context.Context {
	user := auth.User(r)
	userId := user.GetID()
	token := getAuthorizationToken(r)
	if token != "" {
		return context.WithValue(r.Context(), "secCtx", securityContextImpl{
			userId:   userId,
			token:    token,
			apiKey:   "",
			isSystem: false,
		})
	} else {
		return context.WithValue(r.Context(), "secCtx", securityContextImpl{
			userId:   userId,
			token:    "",
			apiKey:   getApihubApiKey(r),
			isSystem: false,
		})
	}
}

func MakeSysadminContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "secCtx", securityContextImpl{isSystem: true})
}

func GetUserContext(ctx context.Context) {

}

// deprecated
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
	if token := getTokenFromAuthHeader(r); token != "" {
		return token
	}
	return getTokenFromCookie(r)
}

func getTokenFromAuthHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return ""
	}
	return strings.TrimSpace(authHeader[7:])
}

func getTokenFromCookie(r *http.Request) string {
	accessTokenCookie, err := r.Cookie("apihub-access-token")
	if err != nil {
		return ""
	}

	return accessTokenCookie.Value
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

func IsSystem(ctx context.Context) bool {
	val := ctx.Value("secCtx")
	if val == nil {
		return false
	}
	return val.(securityContextImpl).isSystem
}

func GetUserId(ctx context.Context) string {
	val := ctx.Value("secCtx")
	if val == nil {
		return ""
	}
	return val.(securityContextImpl).GetUserId()
}

func GetUserToken(ctx context.Context) string {
	val := ctx.Value("secCtx")
	if val == nil {
		return ""
	}
	return val.(securityContextImpl).GetUserToken()
}

func GetApiKey(ctx context.Context) string {
	val := ctx.Value("secCtx")
	if val == nil {
		return ""
	}
	return val.(securityContextImpl).GetApiKey()
}
