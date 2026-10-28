package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/auth"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

const syntheticSigningKey = "synthetic-test-signing-key-not-for-production"

func TestJwtUserTokenRoundTrip(t *testing.T) {
	user := models.User{Username: "synthetic-user", FullName: "Synthetic User", Email: "synthetic@example.invalid", Role: "user"}
	before := time.Now()
	token, err := auth.JwtGenerateFastenTokenFromUser(user, syntheticSigningKey)
	require.NoError(t, err)
	claims, err := auth.JwtValidateFastenToken(syntheticSigningKey, token)
	require.NoError(t, err)
	require.Equal(t, user.Username, claims.Subject)
	require.Equal(t, user.FullName, claims.FullName)
	require.Equal(t, user.Email, claims.Email)
	require.Equal(t, user.Role, claims.Role)
	require.Equal(t, "docker-fastenhealth", claims.Issuer)
	require.Empty(t, claims.TokenType)
	require.Empty(t, claims.ID)
	require.WithinDuration(t, before.Add(time.Hour), claims.ExpiresAt.Time, 2*time.Second)
	require.WithinDuration(t, before, claims.IssuedAt.Time, 2*time.Second)
}

func TestJwtAccessTokenRoundTrip(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	token, err := auth.JwtGenerateAccessToken(models.User{Username: "synthetic-user"}, syntheticSigningKey, expiresAt, "synthetic-token-id")
	require.NoError(t, err)
	claims, err := auth.JwtValidateFastenToken(syntheticSigningKey, token)
	require.NoError(t, err)
	require.Equal(t, "synthetic-user", claims.Subject)
	require.Equal(t, "access", claims.TokenType)
	require.Equal(t, "synthetic-token-id", claims.ID)
	require.Equal(t, expiresAt.Unix(), claims.ExpiresAt.Unix())
}

func TestJwtRejectsEmptySigningKey(t *testing.T) {
	for _, key := range []string{"", " \t\n"} {
		t.Run("key="+key, func(t *testing.T) {
			user := models.User{Username: "synthetic-user"}
			_, err := auth.JwtGenerateFastenTokenFromUser(user, key)
			require.Error(t, err)
			_, err = auth.JwtGenerateAccessToken(user, key, time.Now().Add(time.Hour), "synthetic-id")
			require.Error(t, err)

			// Sign directly: generation guards must not be the only defense.
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
				Subject: user.Username, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			}).SignedString([]byte(key))
			require.NoError(t, err)
			claims, err := auth.JwtValidateFastenToken(key, token)
			require.Error(t, err)
			require.Nil(t, claims)
		})
	}
}

func TestJwtRejectsInvalidTokens(t *testing.T) {
	future := time.Now().Add(time.Hour).Unix()
	past := time.Now().Add(-time.Hour).Unix()
	tests := []struct {
		name   string
		method jwt.SigningMethod
		claims jwt.MapClaims
		key    interface{}
	}{
		{"missing expiry", jwt.SigningMethodHS256, jwt.MapClaims{"sub": "synthetic-user"}, []byte(syntheticSigningKey)},
		{"null expiry", jwt.SigningMethodHS256, jwt.MapClaims{"sub": "synthetic-user", "exp": nil}, []byte(syntheticSigningKey)},
		{"expired", jwt.SigningMethodHS256, jwt.MapClaims{"exp": past}, []byte(syntheticSigningKey)},
		{"not yet valid", jwt.SigningMethodHS256, jwt.MapClaims{"exp": future, "nbf": future}, []byte(syntheticSigningKey)},
		{"issued in future", jwt.SigningMethodHS256, jwt.MapClaims{"exp": future, "iat": future}, []byte(syntheticSigningKey)},
		{"wrong key", jwt.SigningMethodHS256, jwt.MapClaims{"exp": future}, []byte("different-synthetic-key")},
		{"wrong HMAC algorithm", jwt.SigningMethodHS384, jwt.MapClaims{"exp": future}, []byte(syntheticSigningKey)},
		{"unsigned", jwt.SigningMethodNone, jwt.MapClaims{"exp": future}, jwt.UnsafeAllowNoneSignatureType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(tt.method, tt.claims).SignedString(tt.key)
			require.NoError(t, err)
			require.NotPanics(t, func() {
				claims, err := auth.JwtValidateFastenToken(syntheticSigningKey, token)
				require.Error(t, err)
				require.Nil(t, claims)
			})
		})
	}
	for _, token := range []string{"", "not-a-jwt", "a.b.c", strings.Repeat(".", 4096)} {
		claims, err := auth.JwtValidateFastenToken(syntheticSigningKey, token)
		require.Error(t, err)
		require.Nil(t, claims)
	}
}
