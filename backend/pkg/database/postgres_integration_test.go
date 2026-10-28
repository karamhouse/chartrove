//go:build postgres_integration

package database

import (
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

	"github.com/google/uuid"
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
