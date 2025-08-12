package secctx

import (
	"context"
	"github.com/shaj13/go-guardian/v2/auth"
	"net/http"
	"strings"
)

type SecurityContext interface {
	getUserId() string
	getUserToken() string
	getApiKey() string
	IsSystem() bool
}

func MakeUserContext(r *http.Request) context.Context {
	user := auth.User(r)
	userId := user.GetID()

	token := getAuthorizationToken(r)
	apiKey := getApihubApiKey(r)
	pat := getPersonalAccessToken(r)

	return context.WithValue(r.Context(), "secCtx", securityContextImpl{
		userId:              userId,
		token:               token,
		apiKey:              apiKey,
		personalAccessToken: pat,
		isSystem:            false,
	})
}

func MakeSysadminContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "secCtx", securityContextImpl{userId: "system", isSystem: true})
}

type securityContextImpl struct {
	userId              string
	token               string
	apiKey              string
	personalAccessToken string
	isSystem            bool
}

func getAuthorizationToken(r *http.Request) string {
	if token := getTokenFromAuthHeader(r); token != "" {
		return token
	}
	return getTokenFromCookie(r)
}

func getPersonalAccessToken(r *http.Request) string {
	return r.Header.Get("X-Personal-Access-Token")
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
	return val.(securityContextImpl).userId
}

func GetUserToken(ctx context.Context) string {
	val := ctx.Value("secCtx")
	if val == nil {
		return ""
	}
	return val.(securityContextImpl).token
}

func GetApiKey(ctx context.Context) string {
	val := ctx.Value("secCtx")
	if val == nil {
		return ""
	}
	return val.(securityContextImpl).apiKey
}

func GetPersonalAccessToken(ctx context.Context) string {
	val := ctx.Value("secCtx")
	if val == nil {
		return ""
	}
	return val.(securityContextImpl).personalAccessToken
}
