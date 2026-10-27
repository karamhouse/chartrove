package smart

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

type callbackResult struct {
	code   string
	denied bool
}
type callbackListener struct {
	server   *http.Server
	result   chan callbackResult
	ctx      context.Context
	consumed atomic.Bool
}

func (c *callbackListener) close() {
	// A valid callback can reach Connect before net/http flushes its response.
	// Let that response finish; cancellation and unaccepted callbacks close at once.
	if c.consumed.Load() && c.ctx.Err() == nil {
		grace, cancel := context.WithTimeout(context.Background(), time.Second)
		c.server.Shutdown(grace)
		cancel()
	}
	c.server.Close()
}

func listenCallback(ctx context.Context, p Profile, state string) (*callbackListener, error) {
	target, _ := url.Parse(p.RedirectURI)
	listener, err := net.Listen("tcp4", target.Host)
	if err != nil {
		return nil, errors.New("registered SMART callback port is unavailable")
	}
	c := &callbackListener{result: make(chan callbackResult, 1), ctx: ctx}
	c.server = &http.Server{ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	c.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		reject := func() { w.WriteHeader(http.StatusBadRequest); io.WriteString(w, "Invalid authorization callback.") }
		if r.Method != http.MethodGet || r.Host != target.Host || r.URL.Path != target.Path || len(r.RequestURI) > 8192 || ctx.Err() != nil {
			reject()
			return
		}
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			reject()
			return
		}
		if values, ok := q["iss"]; ok && (len(values) != 1 || values[0] != p.Issuer) {
			reject()
			return
		}
		code, errorValue := q.Get("code"), q.Get("error")
		if (len(q["code"]) != 1 || code == "" || len(code) > 4096 || len(q["error"]) != 0) && (len(q["error"]) != 1 || errorValue == "" || len(q["code"]) != 0) {
			reject()
			return
		}
		if !c.consumed.CompareAndSwap(false, true) {
			reject()
			return
		}
		io.WriteString(w, "Authorization received. Return to Chartrove.")
		c.result <- callbackResult{code: code, denied: errorValue != ""}
	})
	go c.server.Serve(listener)
	return c, nil
}
