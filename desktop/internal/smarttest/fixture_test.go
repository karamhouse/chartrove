package smarttest

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/karamhouse/fasten-desktop/internal/smart"
)

func newFixture(t *testing.T) *Fixture {
	t.Helper()
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.Close)
	return f
}

func TestSyntheticFixtureConnectsDistinctSourcesWithSamePatient(t *testing.T) {
	f := newFixture(t)
	m, err := smart.NewManagerWithHTTPClient(f.Profiles(), f.OpenBrowser, f.Client())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ids := map[string]bool{}
	for _, provider := range []string{"synthetic-epic-a", "synthetic-epic-b"} {
		c, err := m.Connect(ctx, provider)
		if err != nil {
			t.Fatal(err)
		}
		if c.ProviderID != provider || c.PatientID != PatientID || ids[c.ID] {
			t.Fatal("synthetic source identity mismatch")
		}
		ids[c.ID] = true
		patient, err := m.ReadPatient(ctx, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		var identity struct {
			ResourceType, ID, BirthDate, Gender string
			Name                                []struct{ Text string }
		}
		if json.Unmarshal([]byte(patient), &identity) != nil || identity.ResourceType != "Patient" || identity.ID != PatientID || identity.BirthDate != "2000-01-02" || identity.Gender != "unknown" || len(identity.Name) != 1 || identity.Name[0].Text != "Chartrove Synthetic Patient" {
			t.Fatal("unexpected synthetic demographics")
		}
		m.Disconnect(c.ID)
		if _, err := m.ReadPatient(ctx, c.ID); err == nil {
			t.Fatal("disconnected synthetic source remained readable")
		}
	}
}

func TestSyntheticFixtureSlowAuthorizationCancels(t *testing.T) {
	f := newFixture(t)
	opened := make(chan struct{})
	m, err := smart.NewManagerWithHTTPClient(f.Profiles(), func(raw string) error {
		err := f.OpenBrowser(raw)
		close(opened)
		return err
	}, f.Client())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	done := make(chan error, 1)
	go func() { _, err := m.Connect(context.Background(), "synthetic-epic-slow"); done <- err }()
	select {
	case <-opened:
	case <-time.After(3 * time.Second):
		t.Fatal("synthetic authorization never opened")
	}
	m.Cancel("synthetic-epic-slow")
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled synthetic authorization connected")
		}
	case <-time.After(time.Second):
		t.Fatal("synthetic authorization did not cancel promptly")
	}
	closed := make(chan struct{})
	go func() { f.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("fixture close did not cancel its slow browser driver")
	}
}

func authorization(t *testing.T, f *Fixture, p smart.Profile, verifier string) string {
	t.Helper()
	u, _ := url.Parse(p.AuthorizationEndpoint)
	sum := sha256.Sum256([]byte(verifier))
	u.RawQuery = url.Values{"response_type": {"code"}, "client_id": {p.ClientID}, "redirect_uri": {p.RedirectURI}, "aud": {p.FHIRBaseURL}, "scope": {strings.Join(p.Scopes, " ")}, "state": {strings.Repeat("s", 43)}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}.Encode()
	client := f.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatal("synthetic authorization rejected valid request")
	}
	callback, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal("invalid synthetic callback")
	}
	return callback.Query().Get("code")
}

func exchange(t *testing.T, f *Fixture, p smart.Profile, code, verifier string) (int, string) {
	t.Helper()
	resp, err := f.Client().PostForm(p.TokenEndpoint, url.Values{"grant_type": {"authorization_code"}, "client_id": {p.ClientID}, "redirect_uri": {p.RedirectURI}, "code": {code}, "code_verifier": {verifier}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var token struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&token)
	return resp.StatusCode, token.AccessToken
}

func TestSyntheticFixtureRejectsInvalidPKCEReplayAndCrossTenantAccess(t *testing.T) {
	f := newFixture(t)
	a, b := f.Profiles()[0], f.Profiles()[1]
	verifier := strings.Repeat("v", 43)
	bad := authorization(t, f, a, verifier)
	if status, _ := exchange(t, f, a, bad, strings.Repeat("x", 43)); status != http.StatusBadRequest {
		t.Fatal("invalid PKCE accepted")
	}
	code := authorization(t, f, a, verifier)
	status, token := exchange(t, f, a, code, verifier)
	if status != http.StatusOK || token == "" {
		t.Fatal("valid PKCE rejected")
	}
	if status, _ := exchange(t, f, a, code, verifier); status != http.StatusBadRequest {
		t.Fatal("authorization code replay accepted")
	}
	code = authorization(t, f, a, verifier)
	if status, _ := exchange(t, f, b, code, verifier); status != http.StatusBadRequest {
		t.Fatal("authorization code accepted by another tenant")
	}
	req, _ := http.NewRequest(http.MethodGet, b.FHIRBaseURL+"/Patient/"+PatientID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := f.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatal("another tenant's token accepted")
	}
	f.mu.Lock()
	f.tokens[token] = accessGrant{providerID: a.ID, expiresAt: time.Now().Add(-time.Second)}
	f.mu.Unlock()
	req, _ = http.NewRequest(http.MethodGet, a.FHIRBaseURL+"/Patient/"+PatientID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	expired, err := f.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer expired.Body.Close()
	if expired.StatusCode != http.StatusUnauthorized {
		t.Fatal("expired synthetic token accepted")
	}
}

func TestSyntheticFixtureConfinesNetworkAndCopiesProfiles(t *testing.T) {
	f := newFixture(t)
	if err := f.OpenBrowser("https://example.invalid/authorize"); err == nil {
		t.Fatal("external browser destination accepted")
	}
	if resp, err := f.Client().Get("https://example.invalid/"); err == nil {
		resp.Body.Close()
		t.Fatal("external network request accepted")
	}
	if resp, err := f.Client().Get(f.Profiles()[0].RedirectURI); err == nil {
		resp.Body.Close()
		t.Fatal("unregistered callback request accepted")
	}
	profiles := f.Profiles()
	profiles[0].Scopes[0] = "changed"
	if f.Profiles()[0].Scopes[0] != "launch/patient" {
		t.Fatal("profile accessor exposed mutable scopes")
	}
	client := f.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	if resp, err := client.Get("http://127.0.0.1:1/"); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		t.Fatal("client override escaped transport boundary")
	}
}
