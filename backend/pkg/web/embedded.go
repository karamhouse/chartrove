package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/event_bus"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// EmbeddedApplication owns a backend without a listener or hosted registration.
// The same handler remains valid when a locked database is unlocked.
type EmbeddedApplication struct {
	engine    *AppEngine
	frontend  http.Handler
	apiBase   string
	routes    atomic.Pointer[gin.Engine]
	lifecycle sync.Mutex
	gate      sync.Mutex
	active    sync.WaitGroup
	closed    bool
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	closeErr  error
}

func NewEmbeddedApplication(cfg config.Interface, logger *logrus.Entry, bus event_bus.Interface, frontend http.Handler) (*EmbeddedApplication, error) {
	if cfg == nil || logger == nil || bus == nil || frontend == nil {
		return nil, errors.New("embedded application requires config, logger, event bus and frontend")
	}
	ctx, cancel := context.WithCancel(context.Background())
	app := &EmbeddedApplication{frontend: frontend, apiBase: strings.TrimRight(cfg.GetString("web.listen.basepath"), "/") + "/api", ctx: ctx, cancel: cancel}
	app.engine = &AppEngine{Config: cfg, Logger: logger, EventBus: bus, embedded: app}
	gin.SetMode(gin.ReleaseMode)
	if err := app.engine.initializeDatabase(); err != nil {
		cancel()
		return nil, err
	}
	app.publishRoutes()
	return app, nil
}

func (app *EmbeddedApplication) publishRoutes() {
	_, router := app.engine.Setup()
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "404 endpoint not found"})
	})
	app.routes.Store(router)
}

func (app *EmbeddedApplication) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.gate.Lock()
	if app.closed {
		app.gate.Unlock()
		http.Error(w, "application is closed", http.StatusServiceUnavailable)
		return
	}
	app.active.Add(1)
	app.gate.Unlock()
	defer app.active.Done()
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(app.ctx, cancel)
	defer stop()
	defer cancel()
	r = r.WithContext(ctx)
	if r.URL.Path == app.apiBase || strings.HasPrefix(r.URL.Path, app.apiBase+"/") {
		app.routes.Load().ServeHTTP(w, r)
		return
	}
	app.frontend.ServeHTTP(w, r)
}

// UnlockDatabase publishes active routes only after the supplied key opens the DB.
// Failed attempts leave the locked routes and previous configuration intact.
func (app *EmbeddedApplication) UnlockDatabase(key string) error {
	app.lifecycle.Lock()
	defer app.lifecycle.Unlock()
	app.gate.Lock()
	closed := app.closed
	app.gate.Unlock()
	if closed {
		return errors.New("application is closed")
	}
	if !app.engine.StandbyMode {
		return errors.New("database is already unlocked")
	}
	if len(key) < 10 {
		return errors.New("encryption key must contain at least 10 characters")
	}
	// Existing requests retain their immutable configuration/router snapshot.
	// Never mutate Viper maps while health/validation requests are reading them.
	settings, err := json.Marshal(app.engine.Config.AllSettings())
	if err != nil {
		return err
	}
	var copied map[string]interface{}
	if err := json.Unmarshal(settings, &copied); err != nil {
		return err
	}
	nextConfig, err := config.Create()
	if err != nil {
		return err
	}
	if err := nextConfig.MergeConfigMap(copied); err != nil {
		return err
	}
	nextConfig.Set("database.encryption.key", key)
	next := &AppEngine{Config: nextConfig, Logger: app.engine.Logger, EventBus: app.engine.EventBus, embedded: app}
	if err := next.initializeDatabase(); err != nil {
		return errors.New("unable to unlock database with the supplied key")
	}
	app.engine = next
	app.publishRoutes()
	return nil
}

// Reinitialize satisfies the existing encryption-handler interface. Embedded
// handlers use UnlockDatabase instead, to keep key changes and route swaps atomic.
func (app *EmbeddedApplication) Reinitialize() error {
	return fmt.Errorf("embedded database must be unlocked with UnlockDatabase")
}

func (app *EmbeddedApplication) Close() error {
	app.closeOnce.Do(func() {
		app.gate.Lock()
		app.closed = true
		app.gate.Unlock()
		app.cancel()
		app.active.Wait()
		app.lifecycle.Lock()
		defer app.lifecycle.Unlock()
		if app.engine.deviceRepo != nil {
			app.closeErr = app.engine.deviceRepo.Close()
		}
	})
	return app.closeErr
}
