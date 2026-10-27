package smart

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncBrowserReceivesCompleteCallbackResponse(t *testing.T) {
	f := newFixture(t)
	browserResult := make(chan bool, 1)
	m := f.manager(t, func(raw string) error {
		q := f.authorization(t, raw)
		target := f.redirect + "?" + url.Values{"state": {q.Get("state")}, "code": {"synthetic-code"}}.Encode()
		go func() {
			client := &http.Client{Timeout: time.Second}
			response, err := client.Get(target)
			if err != nil {
				browserResult <- false
				return
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			browserResult <- err == nil && response.StatusCode == 200 && string(body) == "Authorization received. Return to Chartrove."
		}()
		return nil
	})
	for range 100 {
		c, err := m.Connect(context.Background(), f.profile.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !<-browserResult {
			t.Fatal("asynchronous browser lost the callback confirmation response")
		}
		m.Disconnect(c.ID)
	}
}

func TestCallbackCloseWaitsForResponseHandler(t *testing.T) {
	f := newFixture(t)
	listener, err := listenCallback(context.Background(), f.profile, "expected-state")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.close()
	// Hold the real HTTP handler after it sends the result but before net/http
	// flushes its buffered response, making the native browser scheduling race deterministic.
	original := listener.server.Handler
	handled, release := make(chan struct{}), make(chan struct{})
	listener.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { original.ServeHTTP(w, r); close(handled); <-release })
	received := make(chan bool, 1)
	go func() {
		client := &http.Client{Timeout: time.Second}
		response, err := client.Get(f.profile.RedirectURI + "?state=expected-state&code=synthetic-code")
		if err != nil {
			received <- false
			return
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		received <- err == nil && response.StatusCode == 200 && string(data) == "Authorization received. Return to Chartrove."
	}()
	<-listener.result
	<-handled
	stopped := make(chan struct{})
	go func() { listener.close(); close(stopped) }()
	// The handler is still active when shutdown starts, just as it can be when
	// Connect receives the code on a separate goroutine.
	time.Sleep(30 * time.Millisecond)
	close(release)
	if !<-received {
		t.Error("callback shutdown discarded the browser response")
	}
	<-stopped
}

func connectFixture(t *testing.T, f *fixture) (*Manager, Connection) {
	t.Helper()
	m := f.manager(t, func(raw string) error {
		q := f.authorization(t, raw)
		callback(t, f.redirect, url.Values{"state": {q.Get("state")}, "code": {"synthetic-code"}})
		return nil
	})
	c, err := m.Connect(context.Background(), f.profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	return m, c
}

func TestDiscoveryMismatchDoesNotOpenBrowser(t *testing.T) {
	f := newFixture(t)
	f.discoveredToken = "https://unapproved.example/token"
	var opened atomic.Bool
	m := f.manager(t, func(string) error { opened.Store(true); return nil })
	if _, err := m.Connect(context.Background(), f.profile.ID); err == nil {
		t.Fatal("accepted changed token endpoint")
	}
	if opened.Load() || f.tokenCalls.Load() != 0 {
		t.Fatal("unapproved discovery reached authorization")
	}
}

func TestPatientRedirectNeverReceivesBearer(t *testing.T) {
	f := newFixture(t)
	var requests atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer other.Close()
	f.patientRedirect = other.URL + "/records"
	m, c := connectFixture(t, f)
	if _, err := m.ReadPatient(context.Background(), c.ID); err == nil {
		t.Fatal("accepted FHIR redirect")
	}
	if requests.Load() != 0 {
		t.Fatal("followed token-bearing redirect")
	}
}

func TestPatientIdentityAndExpiryAreEnforced(t *testing.T) {
	f := newFixture(t)
	f.resourceID = "different-patient"
	m, c := connectFixture(t, f)
	if _, err := m.ReadPatient(context.Background(), c.ID); err == nil {
		t.Fatal("accepted different Patient")
	}
	m.mu.Lock()
	stored := m.connections[c.ID]
	stored.summary.ExpiresAt = time.Now().Add(-time.Second).Unix()
	m.connections[c.ID] = stored
	m.mu.Unlock()
	if _, err := m.ReadPatient(context.Background(), c.ID); err == nil {
		t.Fatal("accepted expired connection")
	}
	m.mu.Lock()
	remaining := len(m.connections)
	m.mu.Unlock()
	if remaining != 0 {
		t.Fatal("expired credentials not purged on access")
	}
}

func TestDenialDoesNotExchangeCode(t *testing.T) {
	f := newFixture(t)
	m := f.manager(t, func(raw string) error {
		q := f.authorization(t, raw)
		callback(t, f.redirect, url.Values{"state": {q.Get("state")}, "error": {"access_denied"}, "error_description": {"sensitive-description"}})
		return nil
	})
	_, err := m.Connect(context.Background(), f.profile.ID)
	if err == nil || strings.Contains(err.Error(), "sensitive-description") || f.tokenCalls.Load() != 0 {
		t.Fatal("unsafe authorization denial handling")
	}
}

func TestInsufficientGrantDoesNotCreateConnection(t *testing.T) {
	f := newFixture(t)
	f.scope = "launch/patient"
	m := f.manager(t, func(raw string) error {
		q := f.authorization(t, raw)
		callback(t, f.redirect, url.Values{"state": {q.Get("state")}, "code": {"synthetic-code"}})
		return nil
	})
	if _, err := m.Connect(context.Background(), f.profile.ID); err == nil {
		t.Fatal("accepted grant without Patient permission")
	}
	m.mu.Lock()
	remaining := len(m.connections)
	m.mu.Unlock()
	if remaining != 0 {
		t.Fatal("retained rejected credential")
	}
}

func TestCallbackIsSingleUseAndRejectsAmbiguousResponses(t *testing.T) {
	f := newFixture(t)
	listener, err := listenCallback(context.Background(), f.profile, "expected-state")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.close()
	ambiguous := url.Values{"state": {"expected-state"}, "code": {"synthetic-code"}, "error": {"access_denied"}}
	if callback(t, f.profile.RedirectURI, ambiguous) != 400 {
		t.Fatal("accepted code plus error")
	}
	valid := url.Values{"state": {"expected-state"}, "code": {"synthetic-code"}}
	if callback(t, f.profile.RedirectURI, valid) != 200 {
		t.Fatal("valid callback failed")
	}
	if callback(t, f.profile.RedirectURI, valid) != 400 {
		t.Fatal("accepted replay")
	}
	select {
	case r := <-listener.result:
		if r.code != "synthetic-code" {
			t.Fatal("callback lost")
		}
	default:
		t.Fatal("no callback delivered")
	}
	select {
	case <-listener.result:
		t.Fatal("callback delivered twice")
	default:
	}
}

func TestTimeoutAndBusyProvider(t *testing.T) {
	f := newFixture(t)
	opened := make(chan struct{})
	m := f.manager(t, func(raw string) error { f.authorization(t, raw); close(opened); return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := m.Connect(ctx, f.profile.ID); done <- err }()
	select {
	case <-opened:
	case <-ctx.Done():
		t.Fatal("authorization did not open")
	}
	if _, err := m.Connect(context.Background(), f.profile.ID); err == nil {
		t.Fatal("accepted concurrent authorization for provider")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("authorization did not time out")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout did not release authorization")
	}
	u, _ := url.Parse(f.profile.RedirectURI)
	conn, err := net.DialTimeout("tcp", u.Host, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("timeout leaked listener")
	}
	m.Close()
	if _, err := m.Connect(context.Background(), f.profile.ID); err == nil {
		t.Fatal("closed manager accepted connection")
	}
}
