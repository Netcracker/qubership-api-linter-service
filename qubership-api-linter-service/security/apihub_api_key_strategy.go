package security

import (
	"context"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/client"
	"net/http"

	"github.com/shaj13/go-guardian/v2/auth"
)

func NewApihubApiKeyStrategy(apihubClient client.ApihubClient) auth.Strategy {
	return &apihubApiKeyStrategyImpl{apihubClient: apihubClient}
}

type apihubApiKeyStrategyImpl struct {
	apihubClient client.ApihubClient
}

func (a apihubApiKeyStrategyImpl) Authenticate(ctx context.Context, r *http.Request) (auth.Info, error) {
	apiKeyHeader := r.Header.Get("api-key")
	if apiKeyHeader == "" {
		return nil, fmt.Errorf("authentication failed: %v is empty", "api-key")
	}

	apiKey, err := a.apihubClient.GetApiKeyByKey(apiKeyHeader)
	if err != nil {
		return nil, err
	}
	if apiKey == nil || apiKey.Revoked {
		return nil, fmt.Errorf("authentication failed: %v is not valid", "api-key")
	}
	return auth.NewDefaultUser(apiKey.Name, apiKey.Id, []string{}, auth.Extensions{}), nil
}
