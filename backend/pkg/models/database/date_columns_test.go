package database

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Match the date declarations used by previously generated SQLite models.
type legacyPatientDates struct {
	models.ResourceBase
	Birthdate       *time.Time `gorm:"column:birthdate;type:datetime"`
	MetaLastUpdated *time.Time `gorm:"column:metaLastUpdated;type:datetime"`
}

func (legacyPatientDates) TableName() string { return "fhir_patient" }

func TestFHIRDateColumnsPreserveSQLiteData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "date-columns.db")
	open := func() *gorm.DB {
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   logger.Discard,
		})
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		return db
	}
	db := open()
	require.NoError(t, db.AutoMigrate(&legacyPatientDates{}))
	birthdate := time.Date(2000, 2, 3, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2020, 4, 5, 6, 7, 8, 123000000, time.FixedZone("synthetic", 2*60*60))
	previous := legacyPatientDates{Birthdate: &birthdate, MetaLastUpdated: &updated}
	previous.UserID = uuid.New()
	previous.SourceID = uuid.New()
	previous.SourceResourceType = "Patient"
	previous.SourceResourceID = "synthetic-date"
	require.NoError(t, db.Create(&previous).Error)
	missing := legacyPatientDates{ResourceBase: previous.ResourceBase}
	missing.SourceResourceID = "synthetic-null-date"
	require.NoError(t, db.Create(&missing).Error)

	require.NoError(t, db.AutoMigrate(&FhirPatient{}))
	columns, err := db.Migrator().ColumnTypes(&FhirPatient{})
	require.NoError(t, err)
	dateColumns := 0
	for _, column := range columns {
		if column.Name() == "birthdate" || column.Name() == "metaLastUpdated" {
			require.Equal(t, "datetime", strings.ToLower(column.DatabaseTypeName()))
			dateColumns++
		}
	}
	require.Equal(t, 2, dateColumns)
	pool, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, pool.Close())
	db = open()

	var persisted FhirPatient
	require.NoError(t, db.First(&persisted, "id = ?", previous.ID).Error)
	require.NotNil(t, persisted.Birthdate)
	require.NotNil(t, persisted.MetaLastUpdated)
	require.True(t, birthdate.Equal(*persisted.Birthdate))
	require.True(t, updated.Equal(*persisted.MetaLastUpdated))
	var persistedMissing FhirPatient
	require.NoError(t, db.First(&persistedMissing, "id = ?", missing.ID).Error)
	require.Nil(t, persistedMissing.Birthdate)
	require.Nil(t, persistedMissing.MetaLastUpdated)
}
