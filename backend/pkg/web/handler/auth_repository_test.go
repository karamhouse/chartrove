package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/pkg"
	mock_config "github.com/fastenhealth/fasten-onprem/backend/pkg/config/mock"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/database"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAuthSigninUsesExactUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	pool, err := client.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, client.AutoMigrate(&models.User{}))
	// Keep production password hashing unchanged; this account exists only in memory.
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("synthetic-password"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, client.Create(&models.User{Username: "synthetic-owner", Password: string(passwordHash)}).Error)
	repo := &database.GormRepository{GormClient: client}

	for _, tt := range []struct {
		name   string
		body   map[string]string
		status int
	}{
		{"valid credentials", map[string]string{"username": "synthetic-owner", "password": "synthetic-password"}, http.StatusOK},
		{"empty username", map[string]string{"username": "", "password": "synthetic-password"}, http.StatusUnauthorized},
		{"missing username", map[string]string{"password": "synthetic-password"}, http.StatusUnauthorized},
		{"unknown username", map[string]string{"username": "unknown", "password": "synthetic-password"}, http.StatusUnauthorized},
		{"SQL-looking username", map[string]string{"username": "' OR 1=1 --", "password": "synthetic-password"}, http.StatusUnauthorized},
		{"wrong password", map[string]string{"username": "synthetic-owner", "password": "wrong-password"}, http.StatusUnauthorized},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			appConfig := mock_config.NewMockInterface(ctrl)
			appConfig.EXPECT().GetString("jwt.issuer.key").Return("synthetic-signing-key").AnyTimes()
			body, err := json.Marshal(tt.body)
			require.NoError(t, err)
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Set(pkg.ContextKeyTypeDatabase, repo)
			c.Set(pkg.ContextKeyTypeConfig, appConfig)
			c.Request = httptest.NewRequest(http.MethodPost, "/signin", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.AuthSignin(c)

			require.Equal(t, tt.status, response.Code)
			if tt.status == http.StatusUnauthorized {
				require.JSONEq(t, `{"success":false,"error":"username or password does not match"}`, response.Body.String())
			}
		})
	}
}
