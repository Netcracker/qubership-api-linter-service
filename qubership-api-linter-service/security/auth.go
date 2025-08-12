package security

import (
	"context"
	"crypto/x509"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/secctx"

	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/jwt"
	"github.com/shaj13/go-guardian/v2/auth/strategies/token"
	"github.com/shaj13/go-guardian/v2/auth/strategies/union"
	"github.com/shaj13/libcache"
	_ "github.com/shaj13/libcache/fifo"
	_ "github.com/shaj13/libcache/lru"

	"time"
)

var strategy union.Union
var customJwtStrategy auth.Strategy

const CustomJwtAuthHeader = "X-Apihub-Authorization"

func SetupGoGuardian(apihubClient client.ApihubClient) error {
	if apihubClient == nil {
		return fmt.Errorf("apihubClient is nil")
	}

	ctx := secctx.MakeSysadminContext(context.Background())

	rsaPublicKeyView, err := apihubClient.GetRsaPublicKey(ctx)
	if err != nil {
		return fmt.Errorf("rsa public key error - %s", err.Error())
	}
	if rsaPublicKeyView == nil {
		return fmt.Errorf("rsa public key is empty")
	}

	rsaPublicKey, err := x509.ParsePKCS1PublicKey(rsaPublicKeyView.Value)
	if err != nil {
		return fmt.Errorf("ParsePKCS1PublicKey has error - %s", err.Error())
	}

	keeper := jwt.StaticSecret{
		ID:        "secret-id",
		Secret:    rsaPublicKey,
		Algorithm: jwt.RS256,
	}

	cache := libcache.LRU.New(1000)
	cache.SetTTL(time.Minute * 60)
	cache.RegisterOnExpired(func(key, _ interface{}) {
		cache.Delete(key)
	})

	jwtStrategy := jwt.New(cache, keeper)
	apihubApiKeyStrategy := NewApihubApiKeyStrategy(apihubClient)
	cookieTokenStrategy := NewCookieTokenStrategy(apihubClient)
	patStrategy := NewApihubPATStrategy(apihubClient)
	strategy = union.New(jwtStrategy, apihubApiKeyStrategy, cookieTokenStrategy, patStrategy)

	customJwtStrategy = jwt.New(cache, keeper, token.SetParser(token.XHeaderParser(CustomJwtAuthHeader)))
	return nil
}
