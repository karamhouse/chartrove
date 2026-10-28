//go:build postgres_integration

package database

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/event_bus"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	databaseModels "github.com/fastenhealth/fasten-onprem/backend/pkg/models/database"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgreSQL 17.11. Pin the multi-platform image so the same test runs on arm64
// and amd64. This fixture never accepts an existing database or a host volume.
const postgresTestImage = "postgres:17-alpine@sha256:266d3c257594e195b309ae9d1805e54c788f1d7b375f56bf6c367aac6c917e7b"

func TestPostgresIntegration(t *testing.T) {
	dsn := disposablePostgres(t)

	t.Run("Parameters", func(t *testing.T) {
		for _, mode := range []string{"cache_statement", "simple_protocol"} {
			t.Run(mode, func(t *testing.T) {
				connection := *dsn
				query := connection.Query()
				query.Set("default_query_exec_mode", mode)
				connection.RawQuery = query.Encode()
				db, err := gorm.Open(postgres.Open(connection.String()), &gorm.Config{Logger: logger.Discard})
				require.NoError(t, err)
				pool, err := db.DB()
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, pool.Close()) })

				for _, value := range []string{"synthetic", "synthetic 'quoted' value", "synthetic; SELECT 1; --"} {
					var result struct {
						Literal string
						Value   string
					}
					// The $1 inside the dollar-quoted literal must remain text.
					// This regresses GHSA-j88v-2chj-qfwx through the actual GORM driver.
					err := db.Raw("SELECT $note$literal $1$note$ AS literal, ?::text AS value", value).Scan(&result).Error
					require.NoError(t, err)
					require.Equal(t, "literal $1", result.Literal)
					require.Equal(t, value, result.Value)
				}
			})
		}
	})

	t.Run("RepositoryPersistence", func(t *testing.T) {
		appConfig, err := config.Create()
		require.NoError(t, err)
		appConfig.SetDefault("database.type", "postgres")
		appConfig.SetDefault("database.location", dsn.String())
		appConfig.SetDefault("log.level", "INFO")
		var connectionLogs bytes.Buffer
		quietLogger := logrus.New()
		quietLogger.SetOutput(&connectionLogs)
		repo, err := newTestRepository(t, appConfig, quietLogger, event_bus.NewNoopEventBusServer())
		require.NoError(t, err)
		t.Run("FreshRepositoryHasNoPlaceholderAccount", func(t *testing.T) {
			count, err := repo.GetUserCount(context.Background())
			require.NoError(t, err)
			require.Zero(t, count, "a placeholder account prevents first-user administrator setup")
		})
		t.Run("ConnectionLogsOmitCredentials", func(t *testing.T) {
			password, _ := dsn.User.Password()
			if strings.Contains(connectionLogs.String(), dsn.String()) || strings.Contains(connectionLogs.String(), password) {
				// Do not include the captured log or generated password in test output.
				t.Fatal("connection configuration leaked to application logs")
			}
		})
		user := &models.User{Username: "synthetic_postgres_user", Password: "synthetic-test-password", Email: "synthetic@example.invalid"}
		require.NoError(t, repo.CreateUser(context.Background(), user))
		require.NotEqual(t, uuid.Nil, user.ID)
		require.NotEmpty(t, user.Password)
		require.NotEqual(t, "synthetic-test-password", user.Password)
		updated := time.Date(2020, 4, 5, 6, 7, 8, 123000000, time.UTC)
		patient := databaseModels.FhirPatient{MetaLastUpdated: &updated}
		patient.UserID = user.ID
		patient.SourceID = uuid.New()
		patient.SourceResourceType = "Patient"
		patient.SourceResourceID = "synthetic-postgres-date"
		db := repo.(*GormRepository).GormClient
		require.NoError(t, db.Create(&patient).Error)
		var columnType string
		err = db.Raw("SELECT data_type FROM information_schema.columns WHERE table_name = ? AND column_name = ?", "fhir_patient", "metaLastUpdated").Scan(&columnType).Error
		require.NoError(t, err)
		require.Equal(t, "timestamp with time zone", columnType)
		require.NoError(t, repo.Close())

		reopened, err := newTestRepository(t, appConfig, quietLogger, event_bus.NewNoopEventBusServer())
		require.NoError(t, err)
		t.Run("ReopenDoesNotCreatePlaceholderAccount", func(t *testing.T) {
			count, err := reopened.GetUserCount(context.Background())
			require.NoError(t, err)
			require.Equal(t, 1, count, "reopening must retain only the explicitly created user")
		})
		persisted, err := reopened.GetUserByUsername(context.Background(), user.Username)
		require.NoError(t, err)
		require.Equal(t, user.ID, persisted.ID)
		require.Equal(t, user.Password, persisted.Password)
		var persistedPatient databaseModels.FhirPatient
		require.NoError(t, reopened.(*GormRepository).GormClient.First(&persistedPatient, "id = ?", patient.ID).Error)
		require.NotNil(t, persistedPatient.MetaLastUpdated)
		require.True(t, updated.Equal(*persistedPatient.MetaLastUpdated))
	})
}

func disposablePostgres(t *testing.T) *url.URL {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	if host := os.Getenv("DOCKER_HOST"); host != "" && !strings.HasPrefix(host, "unix://") {
		t.Fatal("PostgreSQL integration tests require a local Docker Unix socket")
	}
	endpoint := strings.TrimSpace(dockerTestOutput(t, ctx, nil, "context", "inspect", "--format", `{{(index .Endpoints "docker").Host}}`))
	if !strings.HasPrefix(endpoint, "unix://") {
		t.Fatal("PostgreSQL integration tests refuse a remote Docker context")
	}

	password := uuid.NewString()
	name := "chartrove-postgres-test-" + uuid.NewString()
	container := strings.TrimSpace(dockerTestOutput(t, ctx, []string{"POSTGRES_PASSWORD=" + password},
		"run", "--detach", "--rm", "--pull=never", "--name", name,
		"--label", "com.chartrove.test=postgres", "--memory=768m", "--cpus=2",
		"--publish", "127.0.0.1::5432", "--tmpfs", "/var/lib/postgresql/data:rw,noexec,nosuid,size=512m",
		"--env", "POSTGRES_USER=chartrove_test", "--env", "POSTGRES_DB=chartrove_test",
		"--env", "POSTGRES_PASSWORD", postgresTestImage))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if out, err := exec.CommandContext(cleanupCtx, "docker", "rm", "--force", container).CombinedOutput(); err != nil {
			t.Errorf("remove disposable PostgreSQL container %s: %v: %s", container, err, out)
		}
	})

	for {
		// Specify TCP so initdb's temporary Unix-socket server is not mistaken
		// for the final server that accepts the published connection.
		err := exec.CommandContext(ctx, "docker", "exec", container, "pg_isready", "-h", "127.0.0.1", "-U", "chartrove_test", "-d", "chartrove_test").Run()
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("disposable PostgreSQL did not become ready within two minutes")
		case <-time.After(250 * time.Millisecond):
		}
	}
	address := strings.TrimSpace(dockerTestOutput(t, ctx, nil, "port", container, "5432/tcp"))
	host, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", host)
	portNumber, err := strconv.Atoi(port)
	require.NoError(t, err)
	require.Greater(t, portNumber, 0)
	t.Logf("Disposable PostgreSQL ready on loopback; image %s", postgresTestImage)
	return &url.URL{
		Scheme: "postgres", User: url.UserPassword("chartrove_test", password),
		Host: net.JoinHostPort(host, fmt.Sprint(portNumber)), Path: "/chartrove_test",
		RawQuery: "sslmode=disable&connect_timeout=5",
	}
}

func dockerTestOutput(t *testing.T, ctx context.Context, extraEnv []string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	output, err := cmd.Output()
	if err != nil {
		// Do not print the command environment: it contains a generated password.
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Docker fixture command failed: %v: %s", err, exitErr.Stderr)
		}
		t.Fatalf("Docker fixture command failed: %v", err)
	}
	return string(output)
}
