package database

import (
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/event_bus"
	"github.com/sirupsen/logrus"
)

// Close successful repositories before testing removes their temporary directories.
// Keep constructor errors intact for the encryption rejection tests.
func newTestRepository(t *testing.T, appConfig config.Interface, logger logrus.FieldLogger, eventBus event_bus.Interface) (DatabaseRepository, error) {
	t.Helper()
	repo, err := NewRepository(appConfig, logger, eventBus)
	if repo != nil {
		t.Cleanup(func() {
			if err := repo.Close(); err != nil {
				t.Errorf("close test repository: %v", err)
			}
		})
	}
	return repo, err
}
