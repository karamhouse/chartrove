package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/pkg"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/auth"
	mock_config "github.com/fastenhealth/fasten-onprem/backend/pkg/config/mock"
	mock_database "github.com/fastenhealth/fasten-onprem/backend/pkg/database/mock"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestAuthSignin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Use a cheap hash only for this synthetic fixture; production cost is unchanged.
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("synthetic-password"), bcrypt.MinCost)
	require.NoError(t, err)
	storedUser := &models.User{Username: "synthetic-user", Password: string(passwordHash)}
	for _, tt := range []struct {
		name      string
		user      *models.User
		lookupErr error
		password  string
		status    int
		message   string
	}{
		{"valid credentials", storedUser, nil, "synthetic-password", http.StatusOK, ""},
		{"unknown user", &models.User{}, gorm.ErrRecordNotFound, "synthetic-password", http.StatusUnauthorized, "username or password does not match"},
		{"wrapped not found", nil, fmt.Errorf("lookup failed: %w", gorm.ErrRecordNotFound), "synthetic-password", http.StatusUnauthorized, "username or password does not match"},
		{"nil user", nil, nil, "synthetic-password", http.StatusUnauthorized, "username or password does not match"},
		{"wrong password", storedUser, nil, "wrong-password", http.StatusUnauthorized, "username or password does not match"},
		{"database failure", nil, errors.New("synthetic internal database failure"), "synthetic-password", http.StatusInternalServerError, "could not sign in"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := mock_database.NewMockDatabaseRepository(ctrl)
			appConfig := mock_config.NewMockInterface(ctrl)
			db.EXPECT().GetUserByUsername(gomock.Any(), "synthetic-user").Return(tt.user, tt.lookupErr)
			if tt.status == http.StatusOK {
				appConfig.EXPECT().GetString("jwt.issuer.key").Return("synthetic-signing-key")
			}
			body, err := json.Marshal(models.User{Username: "synthetic-user", Password: tt.password})
			require.NoError(t, err)
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Set(pkg.ContextKeyTypeDatabase, db)
			c.Set(pkg.ContextKeyTypeConfig, appConfig)
			c.Request = httptest.NewRequest(http.MethodPost, "/signin", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.AuthSignin(c)

			require.Equal(t, tt.status, response.Code)
			if tt.status != http.StatusOK {
				expected, err := json.Marshal(gin.H{"success": false, "error": tt.message})
				require.NoError(t, err)
				require.JSONEq(t, string(expected), response.Body.String())
				return
			}
			var result struct {
				Success bool   `json:"success"`
				Token   string `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.True(t, result.Success)
			claims, err := auth.JwtValidateFastenToken("synthetic-signing-key", result.Token)
			require.NoError(t, err)
			require.Equal(t, storedUser.Username, claims.Subject)
		})
	}
}

func TestAuthSignup(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	t.Run("First user should be assigned admin role", func(t *testing.T) {
		mockDB := mock_database.NewMockDatabaseRepository(mockCtrl)
		mockConfig := mock_config.NewMockInterface(mockCtrl)

		mockDB.EXPECT().GetUserCount(gomock.Any()).Return(0, nil)
		mockDB.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Do(func(_ interface{}, user *models.User) {
			assert.Equal(t, pkg.UserRoleAdmin, user.Role)
		}).Return(nil)
		mockConfig.EXPECT().GetString("jwt.issuer.key").Return("test_key")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(pkg.ContextKeyTypeDatabase, mockDB)
		c.Set(pkg.ContextKeyTypeConfig, mockConfig)

		userWizard := handler.UserWizard{
			User: &models.User{
				Username: "testuser",
				Password: "testpass",
			},
		}
		jsonData, _ := json.Marshal(userWizard)
		c.Request, _ = http.NewRequest(http.MethodPost, "/signup", bytes.NewBuffer(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.AuthSignup(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("Subsequent user should be assigned user role", func(t *testing.T) {
		mockDB := mock_database.NewMockDatabaseRepository(mockCtrl)
		mockConfig := mock_config.NewMockInterface(mockCtrl)

		mockDB.EXPECT().GetUserCount(gomock.Any()).Return(1, nil)
		mockDB.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Do(func(_ interface{}, user *models.User) {
			assert.Equal(t, pkg.UserRoleUser, user.Role)
		}).Return(nil)
		mockConfig.EXPECT().GetString("jwt.issuer.key").Return("test_key")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(pkg.ContextKeyTypeDatabase, mockDB)
		c.Set(pkg.ContextKeyTypeConfig, mockConfig)

		userWizard := handler.UserWizard{
			User: &models.User{
				Username: "testuser2",
				Password: "testpass2",
			},
		}
		jsonData, _ := json.Marshal(userWizard)
		c.Request, _ = http.NewRequest(http.MethodPost, "/signup", bytes.NewBuffer(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.AuthSignup(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})
}
