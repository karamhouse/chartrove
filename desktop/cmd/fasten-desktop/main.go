package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/event_bus"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/web"
	"github.com/karamhouse/fasten-desktop/assets"
	"github.com/karamhouse/fasten-desktop/internal/host"
	"github.com/karamhouse/fasten-desktop/internal/shell"
	"github.com/karamhouse/fasten-desktop/internal/smart"
	"github.com/karamhouse/fasten-desktop/internal/smarttest"
	"github.com/karamhouse/fasten-desktop/internal/smoke"
	"github.com/sirupsen/logrus"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	directory := flag.String("data-dir", "", "Private Chartrove data directory (never the purchased app's data)")
	unencrypted := flag.Bool("unencrypted", false, "Use an unencrypted database for disposable testing")
	frontendDir := flag.String("frontend-dir", "", "Override built-in frontend assets for development")
	smokeTest := flag.Bool("smoke-test", false, "Run local WebView smoke checks with an explicit fresh --data-dir and --unencrypted")
	smartConfig := flag.String("smart-provider-config", "", "Explicit public SMART provider JSON configuration (disabled when omitted)")
	smartUITest := flag.Bool("smart-ui-test", false, "Use local synthetic SMART providers; requires a fresh explicit data directory and --unencrypted")
	flag.Parse()
	if *smartUITest {
		if err := validateSMARTUITest(*directory, *unencrypted, *smartConfig, *smokeTest); err != nil {
			return err
		}
	}
	profiles, err := smart.LoadProfiles(*smartConfig)
	if err != nil {
		return err
	}
	var app *application.App
	openBrowser := func(raw string) error { return app.Browser.OpenURL(raw) }
	client := http.DefaultClient
	if *smartUITest {
		fixture, err := smarttest.New()
		if err != nil {
			return err
		}
		defer fixture.Close()
		profiles, openBrowser, client = fixture.Profiles(), fixture.OpenBrowser, fixture.Client()
	}
	connections, err := smart.NewManagerWithHTTPClient(profiles, openBrowser, client)
	if err != nil {
		return err
	}
	defer connections.Close()
	if *smokeTest {
		if len(profiles) != 0 {
			return fmt.Errorf("smoke test requires direct SMART providers to be disabled")
		}
		if *directory == "" || !*unencrypted {
			return fmt.Errorf("smoke test requires explicit --data-dir and --unencrypted")
		}
		if _, err := os.Stat(filepath.Join(*directory, "fasten.db")); !os.IsNotExist(err) {
			return fmt.Errorf("smoke test requires a fresh database directory")
		}
	}
	storage, err := host.OpenStorage(*directory)
	if err != nil {
		return err
	}
	defer storage.Close()
	cfg, err := config.Create()
	if err != nil {
		return err
	}
	cfg.Set("database.location", filepath.Join(storage.Directory, "fasten.db"))
	cfg.Set("database.encryption.enabled", !*unencrypted)
	cfg.Set("database.encryption.key", "")
	cfg.Set("jwt.issuer.key", storage.SigningKey)
	cfg.Set("web.listen.host", "127.0.0.1")
	cfg.Set("web.listen.basepath", "")
	cfg.Set("web.allow_unsafe_endpoints", false)
	cfg.Set("log.level", "WARN")
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	appLogger := logrus.NewEntry(logger)
	var frontendFS fs.FS
	if *frontendDir != "" {
		frontendFS = os.DirFS(*frontendDir)
	} else {
		frontendFS, err = fs.Sub(assets.Frontend, "frontend")
		if err != nil {
			return err
		}
	}
	frontend, err := host.FrontendHandler(frontendFS)
	if err != nil {
		return err
	}
	backend, err := web.NewEmbeddedApplication(cfg, appLogger, event_bus.NewEventBusServer(appLogger), frontend)
	if err != nil {
		return err
	}
	defer backend.Close()
	var session *smoke.Session
	var smokeOK atomic.Bool
	var assetHandler http.Handler = backend
	if *smokeTest {
		session = &smoke.Session{}
		assetHandler = session.Wrap(backend)
	}
	shouldQuit := host.ShouldQuitAfter(func() {
		connections.Close()
		if err := backend.Close(); err != nil {
			log.Print("closing database: ", err)
		}
	}, func() { app.Quit() })
	app = application.New(application.Options{
		Name:        shell.ApplicationName,
		Description: "Chartrove personal health records; an independent fork of Fasten Onprem",
		Assets:      application.AssetOptions{Handler: assetHandler},
		ShouldQuit:  shouldQuit,
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		PostShutdown: func() {
			if *smokeTest && !smokeOK.Load() {
				os.Exit(1)
			}
		},
	})
	app.RegisterService(application.NewService(shell.NewDesktopService(app)))
	app.RegisterService(application.NewService(shell.NewSMARTService(connections)))
	menu := application.NewMenu()
	menu.AddRole(application.AppMenu)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.WindowMenu)
	app.Menu.Set(menu)
	url := "/"
	if session != nil {
		url = "/__desktop_smoke__"
		session.OpenObserver = func() {
			app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "auth-observer", Title: "Synthetic callback observer", URL: "/__desktop_smoke__?role=observer", Width: 600, Height: 400})
		}
		session.OpenCallback = func() {
			app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "auth-smoke-one", Title: "Synthetic callback", URL: "/__desktop_smoke__?role=callback", Width: 600, Height: 400})
		}
		session.CheckObserver = func() {
			if window, ok := app.Window.GetByName("auth-observer"); ok {
				window.ExecJS("window.reportSmokeStatus()")
			}
		}
		var completed atomic.Bool
		session.Complete = func(report smoke.Report) {
			if !completed.CompareAndSwap(false, true) {
				return
			}
			smokeOK.Store(report.OK)
			b, _ := json.Marshal(report)
			fmt.Println("DESKTOP_SMOKE " + string(b))
			app.Quit()
		}
		go func() {
			time.Sleep(45 * time.Second)
			session.Complete(smoke.Report{Error: "native smoke test timed out"})
		}()
	}
	app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "main", Title: shell.ApplicationName, Width: 1280, Height: 900, MinWidth: 900, MinHeight: 650, URL: url})
	fmt.Printf("%s %s; data directory: %s\n", shell.ApplicationName, shell.Version, storage.Directory)
	return app.Run()
}

func validateSMARTUITest(directory string, unencrypted bool, providerConfig string, smoke bool) error {
	if directory == "" || !unencrypted || providerConfig != "" || smoke {
		return fmt.Errorf("SMART UI testing requires fresh explicit --data-dir, --unencrypted, no provider config, and no smoke mode")
	}
	entries, err := os.ReadDir(directory)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot validate synthetic SMART data directory")
	}
	if len(entries) != 0 {
		return fmt.Errorf("SMART UI testing refuses an existing nonempty data directory")
	}
	return nil
}
