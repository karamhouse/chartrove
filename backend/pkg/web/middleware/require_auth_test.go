package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg"
	mock_config "github.com/fastenhealth/fasten-onprem/backend/pkg/config/mock"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/web/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestRequireAuthRejectsInvalidTokenBeforeProtectedHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name   string
		key    string
		expiry *jwt.NumericDate
		status int
	}{
		{"valid", "synthetic-signing-key", jwt.NewNumericDate(time.Now().Add(time.Hour)), http.StatusNoContent},
		{"missing expiry", "synthetic-signing-key", nil, http.StatusUnauthorized},
		{"expired", "synthetic-signing-key", jwt.NewNumericDate(time.Now().Add(-time.Hour)), http.StatusUnauthorized},
		{"empty signing key", "", jwt.NewNumericDate(time.Now().Add(time.Hour)), http.StatusUnauthorized},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := mock_config.NewMockInterface(gomock.NewController(t))
			config.EXPECT().GetString("jwt.issuer.key").Return(tt.key)
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
				Subject: "synthetic-user", ExpiresAt: tt.expiry,
			}).SignedString([]byte(tt.key))
			require.NoError(t, err)

			called := false
			router := gin.New()
			router.Use(gin.Recovery(), func(c *gin.Context) { c.Set(pkg.ContextKeyTypeConfig, config) }, middleware.RequireAuth())
			router.GET("/protected", func(c *gin.Context) {
				called = true
				require.Equal(t, "synthetic-user", c.GetString(pkg.ContextKeyTypeAuthUsername))
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, tt.status, response.Code)
			require.Equal(t, tt.status == http.StatusNoContent, called)
		})
	}
}
