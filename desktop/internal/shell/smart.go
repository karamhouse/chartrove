package shell

import (
	"context"

	"github.com/karamhouse/fasten-desktop/internal/host"
	"github.com/karamhouse/fasten-desktop/internal/smart"
)

// SMARTService is a separate, main-window-only boundary. Provider windows can
// neither launch connections nor retrieve Patient data or credentials.
type SMARTService struct{ manager *smart.Manager }

func NewSMARTService(manager *smart.Manager) *SMARTService { return &SMARTService{manager: manager} }
func authorizeSMART(ctx context.Context) error             { return host.AuthorizeWindow(caller(ctx), "info", "") }

func (s *SMARTService) Providers(ctx context.Context) ([]smart.ProviderInfo, error) {
	if err := authorizeSMART(ctx); err != nil {
		return nil, err
	}
	return s.manager.Providers(), nil
}
func (s *SMARTService) Connect(ctx context.Context, providerID string) (smart.Connection, error) {
	if err := authorizeSMART(ctx); err != nil {
		return smart.Connection{}, err
	}
	return s.manager.Connect(ctx, providerID)
}
func (s *SMARTService) ReadPatient(ctx context.Context, connectionID string) (string, error) {
	if err := authorizeSMART(ctx); err != nil {
		return "", err
	}
	return s.manager.ReadPatient(ctx, connectionID)
}
func (s *SMARTService) Cancel(ctx context.Context, providerID string) error {
	if err := authorizeSMART(ctx); err != nil {
		return err
	}
	s.manager.Cancel(providerID)
	return nil
}
func (s *SMARTService) Disconnect(ctx context.Context, connectionID string) error {
	if err := authorizeSMART(ctx); err != nil {
		return err
	}
	s.manager.Disconnect(connectionID)
	return nil
}
