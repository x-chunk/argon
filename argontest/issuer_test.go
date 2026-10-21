package argontest

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestFakeIssuerMetadata(t *testing.T) {
	f := NewFakeIssuer(t)
	resp, err := http.Get(f.URL + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var md map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&md); err != nil {
		t.Fatal(err)
	}
	if md["issuer"] != f.URL || md["jwks_uri"] == "" || md["introspection_endpoint"] == "" {
		t.Fatalf("metadata = %v", md)
	}
	if tok := f.Token("u", "aud"); tok == "" {
		t.Fatal("empty token")
	}
}
