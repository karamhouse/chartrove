// Package smarttest provides plaintext synthetic test data for explicit native
// QA. It has no access to operator registrations or existing patient records.
package smarttest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/karamhouse/fasten-desktop/internal/smart"
)

const PatientID = "erSyntheticPatient"

type grant struct {
	profile          smart.Profile
	challenge, scope string
}

type accessGrant struct {
	providerID string
	expiresAt  time.Time
}

type Fixture struct {
	server    *httptest.Server
	profiles  []smart.Profile
	client    *http.Client
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	codes     map[string]grant
	tokens    map[string]accessGrant
	callbacks map[string]bool
	closed    bool
	active    sync.WaitGroup
	once      sync.Once
}

// New starts a private TLS server and chooses three distinct IPv4 callback ports.
func New() (*Fixture, error) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &Fixture{ctx: ctx, cancel: cancel, codes: map[string]grant{}, tokens: map[string]accessGrant{}, callbacks: map[string]bool{}}
	listeners := []net.Listener{}
	defer func() {
		for _, listener := range listeners {
			listener.Close()
		}
	}()
	for range 3 {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			cancel()
			return nil, errors.New("synthetic callback port unavailable")
		}
		listeners = append(listeners, listener)
	}
	f.server = httptest.NewUnstartedServer(http.HandlerFunc(f.serveHTTP))
	f.server.Config.ErrorLog = log.New(io.Discard, "", 0)
	f.server.StartTLS()
	for i, tenant := range []string{"a", "b", "slow"} {
		base := f.server.URL + "/" + tenant
		f.profiles = append(f.profiles, smart.Profile{ID: "synthetic-epic-" + tenant, Display: "Synthetic Epic " + strings.ToUpper(tenant) + " (test data)", FHIRBaseURL: base + "/fhir", Issuer: base, AuthorizationEndpoint: base + "/authorize", TokenEndpoint: base + "/token", ClientID: "chartrove-synthetic-public", RedirectURI: "http://" + listeners[i].Addr().String() + "/smart/callback", Scopes: []string{"launch/patient", "patient/Patient.read"}})
	}
	trusted := f.server.Client()
	f.client = &http.Client{Transport: boundedTransport{fixture: f, inner: trusted.Transport}, Timeout: 15 * time.Second, CheckRedirect: f.redirect}
	return f, nil
}

func (f *Fixture) Profiles() []smart.Profile {
	result := slices.Clone(f.profiles)
	for i := range result {
		result[i].Scopes = slices.Clone(result[i].Scopes)
	}
	return result
}

// Client trusts only the fixture's TLS root. The transport also rejects all
// destinations except this fixture and callbacks minted by its authorization.
func (f *Fixture) Client() *http.Client { clone := *f.client; return &clone }

type boundedTransport struct {
	fixture *Fixture
	inner   http.RoundTripper
}

func (t boundedTransport) CloseIdleConnections() {
	if transport, ok := t.inner.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

func (t boundedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f := t.fixture
	u, err := url.Parse(f.server.URL)
	if err != nil {
		return nil, errors.New("invalid synthetic origin")
	}
	f.mu.Lock()
	allowed := !f.closed && r.URL.User == nil && r.URL.Fragment == "" && ((r.URL.Scheme == u.Scheme && r.URL.Host == u.Host) || (r.Method == http.MethodGet && f.callbacks[r.URL.String()]))
	f.mu.Unlock()
	if !allowed {
		return nil, errors.New("synthetic fixture refuses destination")
	}
	return t.inner.RoundTrip(r)
}

func (f *Fixture) redirect(r *http.Request, via []*http.Request) error {
	if len(via) != 1 {
		return errors.New("synthetic redirect refused")
	}
	f.mu.Lock()
	allowed := f.callbacks[r.URL.String()]
	f.mu.Unlock()
	if !allowed {
		return errors.New("synthetic redirect refused")
	}
	for _, p := range f.profiles {
		if via[0].URL.Scheme+"://"+via[0].URL.Host+via[0].URL.Path == p.AuthorizationEndpoint {
			return nil
		}
	}
	return errors.New("synthetic redirect refused")
}

// OpenBrowser drives only the local synthetic authorization; it never launches
// a real browser or follows an operator-supplied URL.
func (f *Fixture) OpenBrowser(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Fragment != "" {
		return errors.New("synthetic authorization URL refused")
	}
	matched := false
	for _, p := range f.profiles {
		if u.Scheme+"://"+u.Host+u.Path == p.AuthorizationEndpoint {
			matched = true
		}
	}
	if !matched {
		return errors.New("synthetic authorization origin refused")
	}
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return errors.New("synthetic fixture closed")
	}
	f.active.Add(1)
	f.mu.Unlock()
	go func() {
		defer f.active.Done()
		req, err := http.NewRequestWithContext(f.ctx, http.MethodGet, raw, nil)
		if err != nil {
			return
		}
		resp, err := f.client.Do(req)
		if err == nil {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
		}
	}()
	return nil
}

func opaque() string {
	var b [32]byte
	rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func (f *Fixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	for _, p := range f.profiles {
		issuer, _ := url.Parse(p.Issuer)
		prefix := issuer.Path
		switch r.URL.Path {
		case prefix + "/fhir/.well-known/smart-configuration", prefix + "/fhir/metadata":
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"issuer": p.Issuer, "authorization_endpoint": p.AuthorizationEndpoint, "token_endpoint": p.TokenEndpoint, "code_challenge_methods_supported": []string{"S256"}, "grant_types_supported": []string{"authorization_code"}, "capabilities": []string{"client-public", "launch-standalone"}})
			return
		case prefix + "/authorize":
			f.authorize(w, r, p)
			return
		case prefix + "/token":
			f.token(w, r, p)
			return
		case prefix + "/fhir/Patient/" + PatientID:
			f.mu.Lock()
			access := f.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
			f.mu.Unlock()
			if r.Method != http.MethodGet || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || access.providerID != p.ID || !time.Now().Before(access.expiresAt) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/fhir+json")
			json.NewEncoder(w).Encode(map[string]any{"resourceType": "Patient", "id": PatientID, "name": []any{map[string]string{"text": "Chartrove Synthetic Patient"}}, "birthDate": "2000-01-02", "gender": "unknown"})
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (f *Fixture) authorize(w http.ResponseWriter, r *http.Request, p smart.Profile) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	challenge, decodeErr := base64.RawURLEncoding.DecodeString(q.Get("code_challenge"))
	valid := err == nil && decodeErr == nil && len(challenge) == sha256.Size && r.Method == http.MethodGet
	expected := map[string]string{"response_type": "code", "client_id": p.ClientID, "redirect_uri": p.RedirectURI, "aud": p.FHIRBaseURL, "code_challenge_method": "S256", "scope": strings.Join(p.Scopes, " ")}
	for key, value := range expected {
		valid = valid && len(q[key]) == 1 && q.Get(key) == value
	}
	valid = valid && len(q["state"]) == 1 && len(q.Get("state")) >= 32 && len(q["code_challenge"]) == 1
	if !valid {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if p.ID == "synthetic-epic-slow" {
		select {
		case <-r.Context().Done():
			return
		case <-f.ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
	code := opaque()
	callback, _ := url.Parse(p.RedirectURI)
	callback.RawQuery = url.Values{"code": {code}, "state": {q.Get("state")}, "iss": {p.Issuer}}.Encode()
	f.mu.Lock()
	f.codes[code] = grant{profile: p, challenge: q.Get("code_challenge"), scope: q.Get("scope")}
	f.callbacks[callback.String()] = true
	f.mu.Unlock()
	http.Redirect(w, r, callback.String(), http.StatusFound)
}

func (f *Fixture) token(w http.ResponseWriter, r *http.Request, p smart.Profile) {
	if r.Method != http.MethodPost || r.ParseForm() != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	g, ok := f.codes[r.PostForm.Get("code")]
	delete(f.codes, r.PostForm.Get("code"))
	f.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	valid := ok && g.profile.ID == p.ID && g.challenge == base64.RawURLEncoding.EncodeToString(sum[:])
	for key, value := range map[string]string{"grant_type": "authorization_code", "client_id": p.ClientID, "redirect_uri": p.RedirectURI} {
		valid = valid && len(r.PostForm[key]) == 1 && r.PostForm.Get(key) == value
	}
	if !valid || len(r.PostForm["code_verifier"]) != 1 || len(r.PostForm["code"]) != 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	access := opaque()
	f.mu.Lock()
	f.tokens[access] = accessGrant{providerID: p.ID, expiresAt: time.Now().Add(300 * time.Second)}
	f.mu.Unlock()
	json.NewEncoder(w).Encode(map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 300, "patient": PatientID, "scope": g.scope})
}

func (f *Fixture) Close() {
	f.once.Do(func() {
		f.mu.Lock()
		f.closed = true
		f.cancel()
		f.mu.Unlock()
		f.active.Wait()
		f.server.Close()
		f.client.CloseIdleConnections()
	})
}
