package controller

import (
	"context"
	"net/http"

	"github.com/Netcracker/qubership-api-linter-service/secctx"
	"github.com/Netcracker/qubership-api-linter-service/service"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type MCPController interface {
	MakeMCPServer() http.Handler
}

type mcpControllerImpl struct {
	mcpService service.MCPService
}

func (m *mcpControllerImpl) MakeMCPServer() http.Handler {
	return mcpserver.NewStreamableHTTPServer(
		m.mcpService.MakeMCPServer(),
		mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			return secctx.MakeUserContext(r)
		}),
	)
}

func NewMCPController(mcpService service.MCPService) MCPController {
	return &mcpControllerImpl{mcpService: mcpService}
}
