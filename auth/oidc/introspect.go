package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"go.xchunk.org/argon/auth"
)

// introspect validates an opaque token via RFC 7662. Besides "active", the
// response must name our audience: introspection responses are not bound to
// a resource otherwise, and accepting tokens minted for other resources is
// exactly what the mandatory audience is meant to prevent.
func (v *verifier) introspect(ctx context.Context, token string) (*auth.Identity, error) {
	form := url.Values{"token": {token}, "token_type_hint": {"access_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.introspectURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	// RFC 6749 §2.3.1: credentials are form-encoded before Basic auth.
	req.SetBasicAuth(url.QueryEscape(v.cfg.introspect.clientID), url.QueryEscape(v.cfg.introspect.clientSecret))

	resp, err := v.cfg.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("introspection: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes))
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, fmt.Errorf("introspection: %w", err)
	}
	if err := v.checkIntrospected(claims); err != nil {
		return nil, err
	}
	return v.identity(claims), nil
}

func (v *verifier) checkIntrospected(c map[string]any) error {
	if active, _ := c["active"].(bool); !active {
		return errors.New("token is not active")
	}
	if iss, ok := c["iss"]; ok && iss != v.issuer {
		return errors.New("issuer mismatch")
	}
	if !slices.Contains(audiences(c["aud"]), v.audience) {
		return errors.New("audience mismatch or missing")
	}
	now := v.now()
	if exp, ok := c["exp"].(float64); ok && now.After(time.Unix(int64(exp), 0).Add(v.cfg.clockSkew)) {
		return errors.New("token expired")
	}
	if nbf, ok := c["nbf"].(float64); ok && now.Add(v.cfg.clockSkew).Before(time.Unix(int64(nbf), 0)) {
		return errors.New("token not yet valid")
	}
	return nil
}

func audiences(v any) []string {
	switch a := v.(type) {
	case string:
		return []string{a}
	case []any:
		out := make([]string, 0, len(a))
		for _, s := range a {
			if s, ok := s.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
