package smart

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// Connection is deliberately credential-free and safe for the local main UI.
type Connection struct {
	ID         string `json:"id"`
	ProviderID string `json:"provider_id"`
	PatientID  string `json:"patient_id"`
	ExpiresAt  int64  `json:"expires_at"`
}

type credential struct {
	summary Connection
	access  string
	ctx     context.Context
	cancel  context.CancelFunc
}

type Manager struct {
	mu          sync.Mutex
	profiles    map[string]Profile
	pending     map[string]context.CancelFunc
	connections map[string]credential
	closed      bool
	active      sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	client      *http.Client
	openBrowser func(string) error
}

func NewManager(profiles []Profile, openBrowser func(string) error) (*Manager, error) {
	return NewManagerWithHTTPClient(profiles, openBrowser, http.DefaultClient)
}

// NewManagerWithHTTPClient permits an explicitly scoped transport (for example,
// a private TLS test root). It clones the client and always enforces the same
// timeout and no-redirect policy; the caller's client is not modified.
func NewManagerWithHTTPClient(profiles []Profile, openBrowser func(string) error, client *http.Client) (*Manager, error) {
	if openBrowser == nil {
		return nil, errors.New("system browser is unavailable")
	}
	if client == nil {
		return nil, errors.New("SMART HTTP client is unavailable")
	}
	strictClient := *client
	strictClient.Timeout = 15 * time.Second
	strictClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{profiles: map[string]Profile{}, pending: map[string]context.CancelFunc{}, connections: map[string]credential{}, ctx: ctx, cancel: cancel, openBrowser: openBrowser, client: &strictClient}
	for _, p := range profiles {
		if err := p.validate(); err != nil {
			cancel()
			return nil, err
		}
		if _, exists := m.profiles[p.ID]; exists {
			cancel()
			return nil, errors.New("duplicate SMART provider identifier")
		}
		p.FHIRBaseURL = strings.TrimRight(p.FHIRBaseURL, "/")
		p.Scopes = slices.Clone(p.Scopes)
		m.profiles[p.ID] = p
	}
	return m, nil
}

func (m *Manager) Providers() []ProviderInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := []ProviderInfo{}
	if m.closed {
		return result
	}
	for _, p := range m.profiles {
		result = append(result, ProviderInfo{p.ID, p.Display, p.FHIRBaseURL})
	}
	slices.SortFunc(result, func(a, b ProviderInfo) int { return strings.Compare(a.ID, b.ID) })
	return result
}

func randomValue() string {
	var b [32]byte
	// crypto/rand.Read is guaranteed to fill the buffer or terminate the process.
	rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func (m *Manager) Connect(parent context.Context, providerID string) (Connection, error) {
	m.mu.Lock()
	p, exists := m.profiles[providerID]
	if m.closed || !exists {
		m.mu.Unlock()
		return Connection{}, errors.New("SMART provider is unavailable")
	}
	if _, busy := m.pending[providerID]; busy {
		m.mu.Unlock()
		return Connection{}, errors.New("SMART authorization is already pending")
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	stop := context.AfterFunc(m.ctx, cancel)
	m.pending[providerID] = cancel
	m.active.Add(1)
	m.mu.Unlock()
	defer func() {
		cancel()
		stop()
		m.mu.Lock()
		delete(m.pending, providerID)
		m.mu.Unlock()
		m.active.Done()
	}()
	if err := m.discover(ctx, p); err != nil {
		return Connection{}, err
	}
	state, verifier := randomValue(), randomValue()
	callback, err := listenCallback(ctx, p, state)
	if err != nil {
		return Connection{}, err
	}
	defer callback.close()
	sum := sha256.Sum256([]byte(verifier))
	auth, _ := url.Parse(p.AuthorizationEndpoint)
	q := url.Values{"response_type": {"code"}, "client_id": {p.ClientID}, "redirect_uri": {p.RedirectURI}, "scope": {strings.Join(p.Scopes, " ")}, "state": {state}, "aud": {p.FHIRBaseURL}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}
	auth.RawQuery = q.Encode()
	if ctx.Err() != nil {
		return Connection{}, errors.New("SMART authorization cancelled or timed out")
	}
	if m.openBrowser(auth.String()) != nil {
		return Connection{}, errors.New("could not open the system browser")
	}
	var result callbackResult
	select {
	case result = <-callback.result:
	case <-ctx.Done():
		return Connection{}, errors.New("SMART authorization cancelled or timed out")
	}
	if result.denied {
		return Connection{}, errors.New("SMART authorization was denied")
	}
	callback.close()
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {p.ClientID}, "redirect_uri": {p.RedirectURI}, "code": {result.code}, "code_verifier": {verifier}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenEndpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	var tokens struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
		Patient     string `json:"patient"`
		Scope       string `json:"scope"`
	}
	if err := m.requestJSON(req, &tokens, 64*1024); err != nil {
		return Connection{}, errors.New("SMART token exchange failed")
	}
	if tokens.AccessToken == "" || len(tokens.AccessToken) > 16384 || strings.ContainsAny(tokens.AccessToken, "\r\n\t ") || !strings.EqualFold(tokens.TokenType, "Bearer") || tokens.ExpiresIn <= 0 || tokens.ExpiresIn > 604800 || !patientIdentifier.MatchString(tokens.Patient) {
		return Connection{}, errors.New("invalid SMART token response")
	}
	granted := p.Scopes
	if tokens.Scope != "" {
		granted = strings.Fields(tokens.Scope)
	}
	patientRead := false
	for _, scope := range granted {
		if !slices.Contains(p.Scopes, scope) {
			return Connection{}, errors.New("unexpected SMART grant scope")
		}
		if scope == "patient/Patient.read" || scope == "patient/Patient.r" || scope == "patient/Patient.rs" {
			patientRead = true
		}
	}
	if !patientRead {
		return Connection{}, errors.New("Patient read permission was not granted")
	}
	summary := Connection{ID: randomValue(), ProviderID: p.ID, PatientID: tokens.Patient, ExpiresAt: time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second).Unix()}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || ctx.Err() != nil {
		return Connection{}, errors.New("SMART authorization cancelled or timed out")
	}
	connectionCtx, connectionCancel := context.WithCancel(m.ctx)
	m.connections[summary.ID] = credential{summary, tokens.AccessToken, connectionCtx, connectionCancel}
	return summary, nil
}

func (m *Manager) discover(ctx context.Context, p Profile) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.FHIRBaseURL+"/.well-known/smart-configuration", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Epic-Client-ID", p.ClientID)
	var discovery struct {
		Issuer                string   `json:"issuer"`
		AuthorizationEndpoint string   `json:"authorization_endpoint"`
		TokenEndpoint         string   `json:"token_endpoint"`
		ChallengeMethods      []string `json:"code_challenge_methods_supported"`
		Grants                []string `json:"grant_types_supported"`
		Capabilities          []string `json:"capabilities"`
	}
	if m.requestJSON(req, &discovery, 64*1024) != nil {
		return errors.New("SMART discovery failed")
	}
	if (discovery.Issuer != "" && discovery.Issuer != p.Issuer) || discovery.AuthorizationEndpoint != p.AuthorizationEndpoint || discovery.TokenEndpoint != p.TokenEndpoint || !slices.Contains(discovery.ChallengeMethods, "S256") || slices.Contains(discovery.ChallengeMethods, "plain") || !slices.Contains(discovery.Grants, "authorization_code") || !slices.Contains(discovery.Capabilities, "client-public") || !slices.Contains(discovery.Capabilities, "launch-standalone") {
		return errors.New("SMART discovery does not match the approved public provider profile")
	}
	return nil
}

func (m *Manager) requestJSON(req *http.Request, target any, limit int64) error {
	resp, err := m.client.Do(req)
	if err != nil {
		return errors.New("SMART request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("SMART request failed")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(data)) > limit || json.Unmarshal(data, target) != nil {
		return errors.New("invalid SMART response")
	}
	return nil
}

// ReadPatient retrieves only the patient bound by the token response. It never
// accepts arbitrary URLs, follows redirects, imports records, or returns tokens.
func (m *Manager) ReadPatient(parent context.Context, id string) (string, error) {
	m.mu.Lock()
	c, exists := m.connections[id]
	if m.closed || !exists {
		m.mu.Unlock()
		return "", errors.New("SMART connection is unavailable")
	}
	if time.Now().Unix() >= c.summary.ExpiresAt {
		delete(m.connections, id)
		c.cancel()
		m.mu.Unlock()
		return "", errors.New("SMART connection expired; reconnect to continue")
	}
	p := m.profiles[c.summary.ProviderID]
	m.active.Add(1)
	m.mu.Unlock()
	defer m.active.Done()
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(c.ctx, cancel)
	defer cancel()
	defer stop()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.FHIRBaseURL+"/Patient/"+url.PathEscape(c.summary.PatientID), nil)
	req.Header.Set("Authorization", "Bearer "+c.access)
	req.Header.Set("Accept", "application/fhir+json")
	var patient json.RawMessage
	if m.requestJSON(req, &patient, 2*1024*1024) != nil {
		return "", errors.New("SMART Patient request failed")
	}
	var identity struct {
		ResourceType string `json:"resourceType"`
		ID           string `json:"id"`
	}
	if json.Unmarshal(patient, &identity) != nil || identity.ResourceType != "Patient" || identity.ID != c.summary.PatientID {
		return "", errors.New("SMART response did not match the authorized Patient")
	}
	if ctx.Err() != nil {
		return "", errors.New("SMART Patient request cancelled")
	}
	return string(patient), nil
}

func (m *Manager) Cancel(providerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel := m.pending[providerID]; cancel != nil {
		cancel()
	}
}

// Disconnect forgets local credentials; it does not claim provider-side revocation.
func (m *Manager) Disconnect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.connections[id]; ok {
		c.cancel()
		delete(m.connections, id)
	}
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.cancel()
	clear(m.connections)
	m.mu.Unlock()
	m.active.Wait()
}
