package host

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestStorageExcludesSecondInstanceAndPersistsSigningKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fresh")
	first, err := OpenStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := OpenStorage(dir); err == nil {
		t.Fatal("second instance obtained the same data directory")
	}
	key := first.SigningKey
	if len(key) < 32 {
		t.Fatal("signing key is too short")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := OpenStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if !bytes.Equal([]byte(key), []byte(second.SigningKey)) {
		t.Fatal("signing key changed across restart")
	}
	stat, err := os.Stat(filepath.Join(dir, "jwt.key"))
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0600 {
		t.Fatal("signing key is not private")
	}
	if _, err := os.Stat(filepath.Join(dir, "fasten.db")); !os.IsNotExist(err) {
		t.Fatal("storage setup should not create the database")
	}
}

func TestStorageRejectsSymlinkDataDirectory(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStorage(link); err == nil {
		t.Fatal("symlink data directory accepted")
	}
}

func TestFrontendServesSPAWithoutMaskingMissingAssets(t *testing.T) {
	h, err := FrontendHandler(fstest.MapFS{"index.html": {Data: []byte("UI")}, "main.js": {Data: []byte("script")}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		path   string
		status int
		body   string
	}{{"/", 200, "UI"}, {"/dashboard", 200, "UI"}, {"/main.js", 200, "script"}, {"/missing.js", 404, ""}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
		if w.Code != tt.status {
			t.Fatalf("%s returned %d", tt.path, w.Code)
		}
		if tt.body != "" && w.Body.String() != tt.body {
			t.Fatalf("%s returned wrong content", tt.path)
		}
	}
	if _, err := FrontendHandler(fstest.MapFS{}); err == nil {
		t.Fatal("missing frontend accepted")
	}
}

func TestExternalURLValidation(t *testing.T) {
	for _, tt := range []struct {
		url   string
		valid bool
	}{{"https://example.org/oauth?state=one", true}, {"http://127.0.0.1:1234/callback", true}, {"https://user:password@example.org", false}, {"http://example.org", false}, {"javascript:alert(1)", false}, {"file:///etc/passwd", false}, {"/relative", false}} {
		err := ValidateExternalURL(tt.url)
		if (err == nil) != tt.valid {
			t.Errorf("URL validation wrong for %q", tt.url)
		}
	}
}

func TestOnlyOwnedWindowsCanInvokeDesktopActions(t *testing.T) {
	for _, tt := range []struct {
		caller, action, state string
		valid                 bool
	}{{"main", "open", "one", true}, {"main", "close", "one", true}, {"auth-one", "complete", "one", true}, {"auth-other", "complete", "one", false}, {"auth-one", "open", "two", false}, {"", "info", "", false}, {"main", "close", "../main", false}} {
		err := AuthorizeWindow(tt.caller, tt.action, tt.state)
		if (err == nil) != tt.valid {
			t.Errorf("authorization wrong for %s/%s/%s", tt.caller, tt.action, tt.state)
		}
	}
}
