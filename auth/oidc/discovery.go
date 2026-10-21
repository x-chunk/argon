package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// metadata is the subset of OIDC / RFC 8414 server metadata Argon needs.
type metadata struct {
	Issuer                string `json:"issuer"`
	JWKSURI               string `json:"jwks_uri"`
	IntrospectionEndpoint string `json:"introspection_endpoint"`
}

// maxMetadataBytes bounds discovery and JWKS response sizes.
const maxMetadataBytes = 1 << 20

// discover fetches metadata for issuer, trying OIDC Discovery first and
// RFC 8414 second. Only URLs derived from the configured issuer are
// contacted, and the returned issuer must match it exactly.
func discover(ctx context.Context, hc *http.Client, issuer string) (*metadata, error) {
	urls, err := wellKnownURLs(issuer)
	if err != nil {
		return nil, err
	}
	var errs []error
	for _, u := range urls {
		var md metadata
		if err := getJSON(ctx, hc, u, &md); err != nil {
			errs = append(errs, err)
			continue
		}
		if md.Issuer != issuer {
			return nil, fmt.Errorf("oidc: metadata issuer %q does not match configured %q", md.Issuer, issuer)
		}
		if md.JWKSURI == "" && md.IntrospectionEndpoint == "" {
			return nil, errors.New("oidc: metadata has neither jwks_uri nor introspection_endpoint")
		}
		return &md, nil
	}
	return nil, fmt.Errorf("oidc: discovery failed: %w", errors.Join(errs...))
}

// wellKnownURLs returns the OIDC and RFC 8414 metadata URLs for issuer.
// OIDC appends the well-known suffix; RFC 8414 inserts it before the path.
func wellKnownURLs(issuer string) ([]string, error) {
	u, err := url.Parse(issuer)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("oidc: invalid issuer URL %q", issuer)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("oidc: issuer URL must not have query or fragment")
	}
	path := strings.TrimSuffix(u.Path, "/")
	base := u.Scheme + "://" + u.Host
	return []string{
		base + path + "/.well-known/openid-configuration",
		base + "/.well-known/oauth-authorization-server" + path,
	}, nil
}

func getJSON(ctx context.Context, hc *http.Client, u string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("GET %s: %w", u, err)
	}
	return nil
}
