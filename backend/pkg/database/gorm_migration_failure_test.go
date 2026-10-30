package database

import (
	"bytes"
	"path/filepath"
	"testing"

	mock_config "github.com/fastenhealth/fasten-onprem/backend/pkg/config/mock"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/event_bus"
	databaseModels "github.com/fastenhealth/fasten-onprem/backend/pkg/models/database"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSqliteStartupRejectsFinalMigrationFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	client, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Discard, DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	pool, err := client.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	var logs bytes.Buffer
	appLogger := logrus.New()
	appLogger.SetOutput(&logs)
	seedRepo := &GormRepository{GormClient: client, Logger: appLogger}
	require.NoError(t, seedRepo.Migrate())
	patient := databaseModels.FhirPatient{}
	patient.UserID = uuid.New()
	patient.SourceID = uuid.New()
	patient.SourceResourceType = "Patient"
	patient.SourceResourceID = "synthetic-migration-failure"
	require.NoError(t, client.Create(&patient).Error)

	// All versioned migrations have run. A view colliding with an empty FHIR
	// table makes only the final schema reconciliation fail in this fixture.
	require.NoError(t, client.Exec("DROP TABLE fhir_account").Error)
	require.NoError(t, client.Exec("CREATE VIEW fhir_account AS SELECT 1 AS id").Error)
	logs.Reset()
	ctrl := gomock.NewController(t)
	appConfig := mock_config.NewMockInterface(ctrl)
	appConfig.EXPECT().GetString("database.location").Return(dbPath).AnyTimes()
	appConfig.EXPECT().GetString("log.level").Return("INFO").AnyTimes()
	appConfig.EXPECT().GetBool("database.encryption.enabled").Return(false).AnyTimes()

	failed, err := newSqliteRepository(appConfig, appLogger, event_bus.NewNoopEventBusServer(), false)
	if failed != nil {
		t.Cleanup(func() { require.NoError(t, failed.Close()) })
	}
	require.Error(t, err, "startup must reject an incomplete FHIR schema")
	require.Nil(t, failed)
	require.NotContains(t, logs.String(), "migration completed successfully")

	// Repair only the disposable collision and verify startup can recover
	// without changing the unrelated synthetic patient record.
	require.NoError(t, client.Exec("DROP VIEW fhir_account").Error)
	recovered, err := newSqliteRepository(appConfig, appLogger, event_bus.NewNoopEventBusServer(), false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, recovered.Close()) })
	var persisted databaseModels.FhirPatient
	require.NoError(t, recovered.(*GormRepository).GormClient.First(&persisted, "id = ?", patient.ID).Error)
	require.Equal(t, patient.SourceResourceID, persisted.SourceResourceID)
	require.Equal(t, patient.UserID, persisted.UserID)
}
