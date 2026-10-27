// Package smart implements Chartrove-owned public SMART connections. It does
// not consult Fasten Lighthouse, bundled catalogs, or any existing database.
package smart

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Profile contains public registration metadata, never provider credentials.
// Endpoints are explicitly pinned by the operator, not supplied by the WebView.
type Profile struct {
	ID                    string   `json:"id"`
	Display               string   `json:"display"`
	FHIRBaseURL           string   `json:"fhir_base_url"`
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	ClientID              string   `json:"client_id"`
	RedirectURI           string   `json:"redirect_uri"`
	Scopes                []string `json:"scopes"`
}

type ProviderInfo struct {
	ID          string `json:"id"`
	Display     string `json:"display"`
	FHIRBaseURL string `json:"fhir_base_url"`
}

var identifier = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
var patientIdentifier = regexp.MustCompile(`^[A-Za-z0-9.-]{1,64}$`)
var readScope = regexp.MustCompile(`^patient/(Patient|[A-Z][A-Za-z0-9]+)\.(read|r|rs)$`)

func httpsURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Opaque == "" && u.RawPath == "" && !strings.ContainsAny(raw, "\r\n\t ")
}

func (p Profile) validate() error {
	invalid := errors.New("invalid public SMART provider profile")
	if !identifier.MatchString(p.ID) || strings.TrimSpace(p.Display) == "" || len(p.Display) > 100 || p.ClientID == "" || len(p.ClientID) > 256 || strings.ContainsAny(p.ClientID, "\r\n\t ") {
		return invalid
	}
	if !httpsURL(p.FHIRBaseURL) || !httpsURL(p.Issuer) || !httpsURL(p.AuthorizationEndpoint) || !httpsURL(p.TokenEndpoint) {
		return invalid
	}
	u, err := url.Parse(p.RedirectURI)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || u.Path == "" || strings.Contains(u.Path, "..") {
		return invalid
	}
	host, port, err := net.SplitHostPort(u.Host)
	n, parseErr := strconv.Atoi(port)
	if err != nil || host != "127.0.0.1" || parseErr != nil || n < 1024 || n > 65535 {
		return invalid
	}
	if len(p.Scopes) < 2 || len(p.Scopes) > 30 || !slices.Contains(p.Scopes, "launch/patient") {
		return invalid
	}
	patientRead := false
	seen := map[string]bool{}
	for _, scope := range p.Scopes {
		if seen[scope] || (scope != "launch/patient" && !readScope.MatchString(scope)) {
			return invalid
		}
		seen[scope] = true
		if scope == "patient/Patient.read" || scope == "patient/Patient.r" || scope == "patient/Patient.rs" {
			patientRead = true
		}
	}
	if !patientRead {
		return invalid
	}
	return nil
}

// LoadProfiles is opt-in: an empty path performs no file reads. Unknown fields
// (including confidential-client credentials) and extra JSON are rejected.
func LoadProfiles(path string) ([]Profile, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("could not open SMART provider configuration")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil || len(data) > 1024*1024 {
		return nil, errors.New("invalid SMART provider configuration")
	}
	var profiles []Profile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(&profiles) != nil || dec.Decode(new(any)) != io.EOF || len(profiles) > 100 {
		return nil, errors.New("invalid SMART provider configuration")
	}
	for _, p := range profiles {
		if err := p.validate(); err != nil {
			return nil, err
		}
	}
	return profiles, nil
}
