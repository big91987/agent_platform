package platform

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpBeforeCallError struct{ error }

func (e *mcpBeforeCallError) Unwrap() error { return e.error }

func mcpNotCalled(err error) bool {
	var before *mcpBeforeCallError
	return errors.As(err, &before) && !errors.Is(err, context.Canceled)
}

// callMCPTool makes one call. A disconnected response is not permission to
// repeat a possibly successful external write. Business failures remain raw
// MCP results; dispatch owns their workflow routing and persistence.
func callMCPTool(ctx context.Context, connection MCPConnection, name string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("MCP tool name is required")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &object); err != nil || object == nil {
		return nil, errors.New("MCP tool arguments must be a JSON object")
	}
	session, err := connectMCP(ctx, connection)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		// Transport errors may contain credential-bearing URLs. Retain a safe
		// outcome classification rather than exposing the underlying error.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("MCP tools/call failed; external outcome may be unknown; inspect before retrying")
	}
	return result, nil
}
