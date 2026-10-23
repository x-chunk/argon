package sdkadapter

import "github.com/modelcontextprotocol/go-sdk/mcp"

func toSDKContent(b Block) mcp.Content {
	switch b.Kind {
	case BlockImage:
		return &mcp.ImageContent{MIMEType: b.MIMEType, Data: b.Data}
	case BlockResourceLink:
		return &mcp.ResourceLink{URI: b.URI, Name: b.Name, Title: b.Title, Description: b.Description, MIMEType: b.MIMEType}
	default:
		return &mcp.TextContent{Text: b.Text}
	}
}

// fromSDKContent converts SDK content back to a Block; ok is false for kinds
// Argon does not model.
func fromSDKContent(c mcp.Content) (b Block, ok bool) {
	switch c := c.(type) {
	case *mcp.TextContent:
		return Block{Kind: BlockText, Text: c.Text}, true
	case *mcp.ImageContent:
		return Block{Kind: BlockImage, MIMEType: c.MIMEType, Data: c.Data}, true
	case *mcp.ResourceLink:
		return Block{Kind: BlockResourceLink, URI: c.URI, Name: c.Name, Title: c.Title, Description: c.Description, MIMEType: c.MIMEType}, true
	}
	return Block{}, false
}
