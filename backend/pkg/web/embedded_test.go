package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/database"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/event_bus"
	"github.com/sirupsen/logrus"
)

func embeddedConfig(t *testing.T, path string, encrypted bool) config.Interface {
	t.Helper()
	cfg, err := config.Create()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Set("database.location", path)
	cfg.Set("database.encryption.enabled", encrypted)
	cfg.Set("jwt.issuer.key", strings.Repeat("test", 16))
	return cfg
}

func openEmbedded(t *testing.T, cfg config.Interface) *EmbeddedApplication {
	t.Helper()
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	app, err := NewEmbeddedApplication(cfg, logrus.NewEntry(logger), event_bus.NewNoopEventBusServer(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "frontend") }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Error(err)
		}
	})
	return app
}

func embeddedRequest(t *testing.T, app http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	return w
}

func TestEmbeddedFreshDatabasePersistsAccountWithoutRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")
	cfg := embeddedConfig(t, path, false)
	app := openEmbedded(t, cfg)
	w := embeddedRequest(t, app, "GET", "/api/health", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"first_run_wizard":true`) {
		t.Fatalf("fresh health: %d %s", w.Code, w.Body)
	}
	payload := `{"username":"desktop-test","password":"synthetic-password","full_name":"Desktop Test","join_mailing_list":false}`
	w = embeddedRequest(t, app, "POST", "/api/auth/signup", payload)
	if w.Code != 200 {
		t.Fatalf("signup: %d %s", w.Code, w.Body)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openEmbedded(t, embeddedConfig(t, path, false))
	w = embeddedRequest(t, reopened, "POST", "/api/auth/signin", payload)
	if w.Code != 200 {
		t.Fatalf("persisted signin: %d %s", w.Code, w.Body)
	}
	w = embeddedRequest(t, reopened, "GET", "/api/health", "")
	if !strings.Contains(w.Body.String(), `"first_run_wizard":false`) {
		t.Fatal(w.Body.String())
	}
}

func TestEmbeddedEncryptionUnlockIsSynchronousAndRejectsWrongKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "encrypted.db")
	key := strings.Repeat("synthetic", 4)
	app := openEmbedded(t, embeddedConfig(t, path, true))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("locked startup created database: %v", err)
	}
	w := embeddedRequest(t, app, "GET", "/api/health", "")
	if !strings.Contains(w.Body.String(), `"standby_mode":true`) {
		t.Fatal(w.Body.String())
	}
	b, _ := json.Marshal(map[string]string{"encryption_key": key})
	w = embeddedRequest(t, app, "POST", "/api/encryption-key", string(b))
	if w.Code != 200 {
		t.Fatalf("unlock: %d %s", w.Code, w.Body)
	}
	w = embeddedRequest(t, app, "GET", "/api/health", "")
	if !strings.Contains(w.Body.String(), `"standby_mode":false`) {
		t.Fatal("unlock returned before new routes were ready", w.Body.String())
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(raw, []byte("SQLite format 3")) {
		t.Fatal("database is not encrypted")
	}
	reopened := openEmbedded(t, embeddedConfig(t, path, true))
	w = embeddedRequest(t, reopened, "POST", "/api/encryption-key", `{"encryption_key":"wrong-synthetic-key"}`)
	if w.Code != 400 {
		t.Fatalf("wrong key accepted: %d", w.Code)
	}
	w = embeddedRequest(t, reopened, "GET", "/api/health", "")
	if !strings.Contains(w.Body.String(), `"standby_mode":true`) {
		t.Fatal("failed unlock replaced locked handler", w.Body.String())
	}
	if err := reopened.UnlockDatabase(key); err != nil {
		t.Fatal("correct key after failure:", err)
	}
}

func TestEmbeddedKeepsAPI404SeparateFromFrontend(t *testing.T) {
	app := openEmbedded(t, embeddedConfig(t, filepath.Join(t.TempDir(), "db"), false))
	for _, path := range []string{"/", "/dashboard"} {
		w := embeddedRequest(t, app, "GET", path, "")
		if w.Code != 200 || w.Body.String() != "frontend" {
			t.Fatalf("frontend %s: %d %s", path, w.Code, w.Body)
		}
	}
	w := embeddedRequest(t, app, "GET", "/api/unknown", "")
	if w.Code != 404 || !strings.Contains(w.Header().Get("Content-Type"), "json") {
		t.Fatalf("API fallback: %d %s", w.Code, w.Body)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	w = embeddedRequest(t, app, "GET", "/api/health", "")
	if w.Code != 503 {
		t.Fatalf("closed handler returned %d", w.Code)
	}
}

func TestEmbeddedCloseCancelsActiveRequests(t *testing.T) {
	cfg := embeddedConfig(t, filepath.Join(t.TempDir(), "db"), false)
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entered := make(chan struct{})
	app, err := NewEmbeddedApplication(cfg, logrus.NewEntry(logger), event_bus.NewNoopEventBusServer(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
	if err != nil {
		t.Fatal(err)
	}
	go embeddedRequest(t, app, "GET", "/blocking", "")
	<-entered
	done := make(chan error, 1)
	go func() { done <- app.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("close did not cancel active request")
	}
}

func TestEmbeddedUnlockWhilePollingHealth(t *testing.T) {
	app := openEmbedded(t, embeddedConfig(t, filepath.Join(t.TempDir(), "db"), true))
	var readers sync.WaitGroup
	for i := 0; i < 4; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for j := 0; j < 50; j++ {
				w := embeddedRequest(t, app, "GET", "/api/health", "")
				if w.Code != 200 {
					t.Errorf("health returned %d", w.Code)
				}
			}
		}()
	}
	if err := app.UnlockDatabase(strings.Repeat("synthetic", 4)); err != nil {
		t.Fatal(err)
	}
	readers.Wait()
}

func TestEmbeddedUnlockDoesNotMutateConfigurationUsedByExistingRequests(t *testing.T) {
	cfg := embeddedConfig(t, filepath.Join(t.TempDir(), "db"), true)
	app := openEmbedded(t, cfg)
	if err := app.UnlockDatabase(strings.Repeat("synthetic", 4)); err != nil {
		t.Fatal(err)
	}
	if cfg.GetString("database.encryption.key") != "" {
		t.Fatal("unlock mutated the configuration still held by existing requests")
	}
}

type blockedTransport struct{ requests int }

func (b *blockedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b.requests++
	return nil, fmt.Errorf("network access is disabled in this test")
}

func TestEmbeddedFreshDatabaseImportsSyntheticFHIRWithoutNetworkOrTempFileResidue(t *testing.T) {
	blocked := &blockedTransport{}
	previous := http.DefaultTransport
	http.DefaultTransport = blocked
	t.Cleanup(func() { http.DefaultTransport = previous })
	importDir := t.TempDir()
	t.Setenv("TMPDIR", importDir)
	app := openEmbedded(t, embeddedConfig(t, filepath.Join(t.TempDir(), "db"), false))
	w := embeddedRequest(t, app, "POST", "/api/auth/signup", `{"username":"synthetic-import","password":"synthetic-password","join_mailing_list":false}`)
	if w.Code != 200 {
		t.Fatalf("signup returned %d", w.Code)
	}
	var auth struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "synthetic-import.json")
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(part, `{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"Patient","id":"erXuFYUfucBZaryVksYEcMg3","name":[{"family":"Synthetic","given":["Desktop"]}],"gender":"unknown"}}]}`)
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/secure/source/manual", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+auth.Data)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("synthetic import returned %d: %s", w.Code, w.Body)
	}
	var imported struct {
		Source struct {
			Patient string `json:"patient"`
		} `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Source.Patient != "rXuFYUfucBZaryVksYEcMg3" {
		t.Fatalf("unexpected pinned importer patient key: %q", imported.Source.Patient)
	}
	req = httptest.NewRequest("GET", "/api/secure/resource/fhir?sourceResourceType=Patient", nil)
	req.Header.Set("Authorization", "Bearer "+auth.Data)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "erXuFYUfucBZaryVksYEcMg3") {
		t.Fatalf("imported patient missing: %d %s", w.Code, w.Body)
	}
	if blocked.requests != 0 {
		t.Fatalf("local import attempted %d network requests", blocked.requests)
	}
	files, err := filepath.Glob(filepath.Join(importDir, "synthetic-import.json*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatal("completed import left a temporary document behind")
	}
}

func TestEmbeddedCloseReleasesDatabasePool(t *testing.T) {
	app := openEmbedded(t, embeddedConfig(t, filepath.Join(t.TempDir(), "close.db"), false))
	repo := app.engine.deviceRepo.(*database.GormRepository)
	pool, err := repo.GormClient.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(); err == nil {
		t.Fatal("embedded Close left the database pool open")
	}
}
