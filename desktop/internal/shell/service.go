package shell

import (
	"context"
	"errors"

	"github.com/karamhouse/fasten-desktop/internal/host"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const Version = "0.1.0-dev"
const ApplicationName = "Chartrove"

type DesktopService struct{ app *application.App }

func NewDesktopService(app *application.App) *DesktopService { return &DesktopService{app: app} }

func caller(ctx context.Context) string {
	window, _ := ctx.Value(application.WindowKey).(application.Window)
	if window == nil {
		return ""
	}
	return window.Name()
}

func (s *DesktopService) Info(ctx context.Context) (map[string]string, error) {
	if err := host.AuthorizeWindow(caller(ctx), "info", ""); err != nil {
		return nil, err
	}
	return map[string]string{"application": ApplicationName, "version": Version}, nil
}

func (s *DesktopService) OpenExternal(ctx context.Context, raw, state string) error {
	if err := host.AuthorizeWindow(caller(ctx), "open", state); err != nil {
		return err
	}
	if err := host.ValidateExternalURL(raw); err != nil {
		return err
	}
	if state == "external" {
		return s.app.Browser.OpenURL(raw)
	}
	name := "auth-" + state
	if _, exists := s.app.Window.GetByName(name); exists {
		return errors.New("authentication window already exists")
	}
	s.app.Window.NewWithOptions(application.WebviewWindowOptions{Name: name, Title: "Connect a healthcare provider", URL: raw, Width: 900, Height: 750})
	return nil
}

func (s *DesktopService) CloseAuthWindow(ctx context.Context, state string) error {
	if err := host.AuthorizeWindow(caller(ctx), "close", state); err != nil {
		return err
	}
	if window, ok := s.app.Window.GetByName("auth-" + state); ok {
		window.Close()
	}
	return nil
}

func (s *DesktopService) CompleteAuth(ctx context.Context, data map[string]string) error {
	state := data["state"]
	if err := host.AuthorizeWindow(caller(ctx), "complete", state); err != nil {
		return err
	}
	main, ok := s.app.Window.GetByName("main")
	if !ok {
		return errors.New("main application window is unavailable")
	}
	// Global event emission would also disclose callback data to other provider
	// windows. Deliver this response only to the local application window.
	main.DispatchWailsEvent(&application.CustomEvent{Name: "fasten:source-auth-response", Data: data})
	if window, ok := s.app.Window.GetByName("auth-" + state); ok {
		window.Close()
	}
	return nil
}
