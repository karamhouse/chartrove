package host

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func FrontendHandler(root fs.FS) (http.Handler, error) {
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return nil, fmt.Errorf("build the desktop frontend first: %w", err)
	}
	server := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		stat, err := fs.Stat(root, name)
		if err != nil || stat.IsDir() {
			assetExtension := map[string]bool{".js": true, ".mjs": true, ".css": true, ".map": true, ".png": true, ".jpg": true, ".svg": true, ".json": true, ".html": true, ".ico": true, ".woff": true, ".woff2": true}
			if errors.Is(err, fs.ErrNotExist) && !strings.HasPrefix(name, "assets/") && !assetExtension[path.Ext(name)] {
				cloned := r.Clone(r.Context())
				cloned.URL.Path = "/"
				server.ServeHTTP(w, cloned)
				return
			}
			http.NotFound(w, r)
			return
		}
		server.ServeHTTP(w, r)
	}), nil
}

// ValidateExternalURL prevents non-web schemes and credential-bearing URLs from
// being passed to a native browser or authentication window.
func ValidateExternalURL(raw string) error {
	return validateExternalURL(raw)
}
