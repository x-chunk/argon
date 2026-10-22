package sdkadapter

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
)

// identityKey is the TokenInfo.Extra key under which the Argon identity
// travels through the SDK's HTTP auth middleware.
const identityKey = "argon.identity"

func identityFromExtra(extra *mcp.RequestExtra) *auth.Identity {
	if extra == nil || extra.TokenInfo == nil {
		return nil
	}
	id, _ := extra.TokenInfo.Extra[identityKey].(*auth.Identity)
	return id
}
