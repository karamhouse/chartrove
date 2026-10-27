package smart

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Removing PKCE verification, endpoint pinning, callback validation, or bearer
// confinement must break these real HTTP exchanges with the synthetic provider.
type fixture struct {
	server          *httptest.Server
	profile         Profile
	tokenCalls      atomic.Int32
	challenge       string
	redirect        string
	tokenStatus     int
	expires         int
	patientID       string
	scope           string
	discoveredToken string
	patientRedirect string
	resourceID      string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	callback := "http://" + listener.Addr().String() + "/oauth/callback"
	listener.Close()
	f := &fixture{expires: 3600, patientID: "synthetic-patient", scope: "launch/patient patient/Patient.read"}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fhir/.well-known/smart-configuration":
			if r.Header.Get("Authorization") != "" {
				t.Error("credential sent to discovery")
			}
			tokenEndpoint := f.server.URL + "/token"
			if f.discoveredToken != "" {
				tokenEndpoint = f.discoveredToken
			}
			json.NewEncoder(w).Encode(map[string]any{"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/authorize", "token_endpoint": tokenEndpoint, "grant_types_supported": []string{"authorization_code"}, "code_challenge_methods_supported": []string{"S256"}, "capabilities": []string{"launch-standalone", "client-public"}})
		case "/token":
			f.tokenCalls.Add(1)
			r.ParseForm()
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
				t.Error("PKCE verifier did not match challenge")
			}
			if r.Form.Get("code") != "synthetic-code" || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("client_id") != "chartrove-test-client" || r.Form.Get("redirect_uri") != f.redirect {
				t.Error("incorrect token exchange binding")
			}
			if r.Header.Get("Authorization") != "" || r.Form.Has("client_secret") {
				t.Error("public client sent secret")
			}
			if f.tokenStatus != 0 {
				w.WriteHeader(f.tokenStatus)
				fmt.Fprint(w, "sensitive-response-must-not-escape")
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "synthetic-access-value", "refresh_token": "synthetic-discard-value", "token_type": "Bearer", "expires_in": f.expires, "patient": f.patientID, "scope": f.scope})
		case "/fhir/Patient/synthetic-patient":
			if r.Header.Get("Authorization") != "Bearer synthetic-access-value" {
				t.Error("missing FHIR bearer")
			}
			if f.patientRedirect != "" {
				http.Redirect(w, r, f.patientRedirect, http.StatusFound)
				return
			}
			id := "synthetic-patient"
			if f.resourceID != "" {
				id = f.resourceID
			}
			json.NewEncoder(w).Encode(map[string]any{"resourceType": "Patient", "id": id, "name": []map[string]string{{"family": "López"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	f.profile = Profile{ID: "epic-test", Display: "Synthetic Epic", FHIRBaseURL: f.server.URL + "/fhir", Issuer: f.server.URL, AuthorizationEndpoint: f.server.URL + "/authorize", TokenEndpoint: f.server.URL + "/token", ClientID: "chartrove-test-client", RedirectURI: callback, Scopes: []string{"launch/patient", "patient/Patient.read"}}
	return f
}

func (f *fixture) manager(t *testing.T, open func(string) error) *Manager {
	t.Helper()
	m, err := NewManager([]Profile{f.profile}, open)
	if err != nil {
		t.Fatal(err)
	}
	m.client = f.server.Client()
	m.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	t.Cleanup(m.Close)
	return m
}

func (f *fixture) authorization(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil || u.Scheme+"://"+u.Host+u.Path != f.profile.AuthorizationEndpoint {
		t.Fatalf("unexpected authorization URL")
	}
	q := u.Query()
	if q.Get("aud") != f.profile.FHIRBaseURL || q.Get("client_id") != f.profile.ClientID || q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || q.Get("scope") != strings.Join(f.profile.Scopes, " ") {
		t.Fatal("incorrect authorization parameters")
	}
	if len(q.Get("state")) < 43 || len(q.Get("code_challenge")) != 43 || q.Has("code_verifier") {
		t.Fatal("unsafe state or PKCE authorization")
	}
	f.challenge = q.Get("code_challenge")
	f.redirect = q.Get("redirect_uri")
	if f.redirect != f.profile.RedirectURI {
		t.Fatal("redirect URI changed from registration")
	}
	return q
}

func callback(t *testing.T, redirect string, values url.Values) int {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(redirect + "?" + values.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "synthetic-code") {
		t.Fatal("callback echoed code")
	}
	return resp.StatusCode
}

func TestConnectPKCEReadPatientAndDisconnect(t *testing.T) {
	f := newFixture(t)
	m := f.manager(t, func(raw string) error {
		q := f.authorization(t, raw)
		if callback(t, f.redirect, url.Values{"state": {"wrong-state"}, "code": {"synthetic-code"}}) != 400 {
			t.Error("accepted wrong state")
		}
		if callback(t, f.redirect, url.Values{"state": {q.Get("state")}, "iss": {"https://wrong.example"}, "code": {"synthetic-code"}}) != 400 {
			t.Error("accepted wrong issuer")
		}
		if callback(t, f.redirect, url.Values{"state": {q.Get("state"), q.Get("state")}, "code": {"synthetic-code"}}) != 400 {
			t.Error("accepted duplicate state")
		}
		if callback(t, f.redirect, url.Values{"state": {q.Get("state")}, "iss": {f.profile.Issuer}, "code": {"synthetic-code"}}) != 200 {
			t.Error("valid callback rejected")
		}
		return nil
	})
	connection, err := m.Connect(context.Background(), f.profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if connection.PatientID != "synthetic-patient" || connection.ProviderID != f.profile.ID || connection.ID == "" {
		t.Fatal("missing public connection summary")
	}
	encoded, _ := json.Marshal(connection)
	if strings.Contains(string(encoded), "synthetic-access") || strings.Contains(string(encoded), "token") {
		t.Fatal("credential in public result")
	}
	patient, err := m.ReadPatient(context.Background(), connection.ID)
	if err != nil || !strings.Contains(patient, "López") {
		t.Fatalf("Patient retrieval failed: %v", err)
	}
	if f.tokenCalls.Load() != 1 {
		t.Fatal("token exchange was not single-use")
	}
	if _, err := net.DialTimeout("tcp", strings.TrimPrefix(strings.Split(f.redirect, "/oauth/")[0], "http://"), 100*time.Millisecond); err == nil {
		t.Fatal("callback listener remained open")
	}
	m.Disconnect(connection.ID)
	if _, err = m.ReadPatient(context.Background(), connection.ID); err == nil {
		t.Fatal("disconnected credentials remained usable")
	}
}

func TestCancelAndCloseReleasePendingListener(t *testing.T) {
	for _, closeManager := range []bool{false, true} {
		t.Run(fmt.Sprint(closeManager), func(t *testing.T) {
			f := newFixture(t)
			opened := make(chan struct{})
			m := f.manager(t, func(raw string) error { f.authorization(t, raw); close(opened); return nil })
			done := make(chan error, 1)
			go func() { _, err := m.Connect(context.Background(), f.profile.ID); done <- err }()
			select {
			case <-opened:
			case err := <-done:
				t.Fatalf("authorization ended before browser launch: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("authorization did not reach browser launch")
			}
			if closeManager {
				m.Close()
			} else {
				m.Cancel(f.profile.ID)
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled flow succeeded")
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled flow did not drain")
			}
			if f.tokenCalls.Load() != 0 {
				t.Fatal("cancelled flow exchanged tokens")
			}
			u, _ := url.Parse(f.profile.RedirectURI)
			conn, err := net.DialTimeout("tcp", u.Host, 100*time.Millisecond)
			if err == nil {
				conn.Close()
				t.Fatal("callback port leaked")
			}
		})
	}
}

func TestProviderErrorsDoNotExposeBodies(t *testing.T) {
	f := newFixture(t)
	f.tokenStatus = 401
	m := f.manager(t, func(raw string) error {
		q := f.authorization(t, raw)
		callback(t, f.redirect, url.Values{"state": {q.Get("state")}, "code": {"synthetic-code"}})
		return nil
	})
	_, err := m.Connect(context.Background(), f.profile.ID)
	if err == nil || strings.Contains(err.Error(), "sensitive-response") || strings.Contains(err.Error(), "synthetic-code") {
		t.Fatal("unsafe provider error")
	}
}

func TestBrowserFailureIsRedactedAndListenerCloses(t *testing.T) {
	f := newFixture(t)
	m := f.manager(t, func(string) error { return errors.New("sensitive-browser-url") })
	_, err := m.Connect(context.Background(), f.profile.ID)
	if err == nil || strings.Contains(err.Error(), "sensitive-browser-url") {
		t.Fatal("unsafe browser error")
	}
	u, _ := url.Parse(f.profile.RedirectURI)
	conn, err := net.DialTimeout("tcp", u.Host, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("browser error leaked callback")
	}
}

func TestProfilesDisabledByDefaultAndStrict(t *testing.T) {
	profiles, err := LoadProfiles("")
	if err != nil || len(profiles) != 0 {
		t.Fatal("default must not load a catalog")
	}
	f := newFixture(t)
	for _, change := range []func(*Profile){
		func(p *Profile) { p.ClientID = "" }, func(p *Profile) { p.FHIRBaseURL = "http://example.com/fhir" },
		func(p *Profile) { p.RedirectURI = "http://0.0.0.0:53210/callback" }, func(p *Profile) { p.RedirectURI = "http://127.0.0.1:0/callback" },
		func(p *Profile) { p.Scopes = []string{"launch/patient", "patient/Patient.cruds"} },
		func(p *Profile) { p.Scopes = []string{"launch/patient", "patient/Patient.read", "offline_access"} },
	} {
		p := f.profile
		change(&p)
		if _, err := NewManager([]Profile{p}, func(string) error { return nil }); err == nil {
			t.Error("accepted unsafe profile")
		}
	}
	path := filepath.Join(t.TempDir(), "providers.json")
	os.WriteFile(path, []byte(`[{"client_secret":"not-supported"}]`), 0600)
	if _, err := LoadProfiles(path); err == nil {
		t.Fatal("accepted confidential credential configuration")
	}
}
