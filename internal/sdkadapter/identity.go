package sdkadapter

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
)

// identityKey is the TokenInfo.Extra key under which the Argon identity
// travels through the SDK's HTTP auth middleware.
const identityKey = "argon.identity"

func identityFromRequest(req *mcp.CallToolRequest) *auth.Identity {
	if req.Extra == nil || req.Extra.TokenInfo == nil {
		return nil
	}
	id, _ := req.Extra.TokenInfo.Extra[identityKey].(*auth.Identity)
	return id
}
