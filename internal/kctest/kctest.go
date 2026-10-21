// Package kctest helps integration tests talk to the Keycloak instance from
// test/keycloak. Tests using it must carry the "integration" build tag.
package kctest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

// Realm settings matching test/keycloak/realm-argon.json.
const (
	Audience           = "http://localhost:9090"
	ServerClientID     = "argon-mcp"
	ServerClientSecret = "argon-mcp-secret"
)

// Issuer returns the realm issuer URL, honouring ARGON_KEYCLOAK_URL.
// If Keycloak is unreachable the test is skipped with instructions.
func Issuer(t testing.TB) string {
	t.Helper()
	base := os.Getenv("ARGON_KEYCLOAK_URL")
	if base == "" {
		base = "http://localhost:8180"
	}
	iss := strings.TrimSuffix(base, "/") + "/realms/argon"
	resp, err := http.Get(iss + "/.well-known/openid-configuration")
	if err != nil {
		t.Skipf("keycloak not reachable at %s (start it with: docker compose -f test/keycloak/compose.yaml up -d --wait): %v", base, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("keycloak realm not ready: status %d", resp.StatusCode)
	}
	return iss
}

// Token obtains an access token via the client credentials grant.
// Known clients: "argon-test" (Argon audience) and "argon-other".
func Token(t testing.TB, issuer, clientID string, scopes ...string) string {
	t.Helper()
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientID + "-secret"},
	}
	if len(scopes) > 0 {
		form.Set("scope", strings.Join(scopes, " "))
	}
	resp, err := http.PostForm(issuer+"/protocol/openid-connect/token", form)
	if err != nil {
		t.Fatalf("token request: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.AccessToken == "" {
		t.Fatalf("token request: status %d, %q, %v", resp.StatusCode, body.Error, err)
	}
	return body.AccessToken
}
