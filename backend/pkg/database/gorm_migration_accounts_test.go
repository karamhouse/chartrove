package database

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/pkg"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPlaceholderAdminMigrationPreservesRealAccounts(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("synthetic-password"), bcrypt.MinCost)
	require.NoError(t, err)
	for _, tt := range []struct {
		name     string
		password string
		retained bool
	}{
		{"password-bearing admin", string(passwordHash), true},
		{"empty-password placeholder", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{
				Logger: logger.Discard, DisableForeignKeyConstraintWhenMigrating: true,
			})
			require.NoError(t, err)
			pool, err := client.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pool.Close()) })
			quietLogger := logrus.New()
			quietLogger.SetOutput(io.Discard)
			repo := &GormRepository{GormClient: client, Logger: quietLogger}
			require.NoError(t, repo.Migrate())

			admin := models.User{Username: "admin", Password: tt.password, Role: pkg.UserRoleAdmin}
			other := models.User{Username: "synthetic-other", Password: string(passwordHash), Role: pkg.UserRoleUser}
			require.NoError(t, client.Create(&admin).Error)
			require.NoError(t, client.Create(&other).Error)
			// Replay only the historical placeholder-removal migration in this
			// disposable database. All other migration receipts remain intact.
			removed := client.Exec("DELETE FROM migrations WHERE id = ?", "20240212142126")
			require.NoError(t, removed.Error)
			require.EqualValues(t, 1, removed.RowsAffected)
			require.NoError(t, repo.Migrate())

			persisted, err := repo.GetUserByUsername(context.Background(), admin.Username)
			if tt.retained {
				require.NoError(t, err)
				require.Equal(t, admin.ID, persisted.ID)
				require.Equal(t, admin.Password, persisted.Password)
				require.Equal(t, admin.Role, persisted.Role)
				require.NoError(t, persisted.CheckPassword("synthetic-password"))
			} else {
				require.True(t, errors.Is(err, gorm.ErrRecordNotFound))
			}
			persistedOther, err := repo.GetUserByUsername(context.Background(), other.Username)
			require.NoError(t, err)
			require.Equal(t, other.ID, persistedOther.ID)
			require.Equal(t, other.Password, persistedOther.Password)
			require.Equal(t, other.Role, persistedOther.Role)
		})
	}
}
