package models

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/jwk"
	jwtVerifier "github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDynamicClientAssertionRoundTrip(t *testing.T) {
	key, err := jwk.JWKGenerate()
	require.NoError(t, err)
	serialized, err := jwk.JWKSerialize(key)
	require.NoError(t, err)
	// Exercise the persisted JSON shape, without storing any real credentials.
	stored, err := json.Marshal([]map[string]string{serialized})
	require.NoError(t, err)
	credential := SourceCredential{DynamicClientId: "synthetic-client"}
	require.NoError(t, json.Unmarshal(stored, &credential.DynamicClientJWKS))
	modulus, err := base64.RawURLEncoding.DecodeString(serialized["n"])
	require.NoError(t, err)
	exponent, err := base64.RawURLEncoding.DecodeString(serialized["e"])
	require.NoError(t, err)
	publicKey := &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(new(big.Int).SetBytes(exponent).Int64())}
	endpoint := "https://provider.example.invalid/token"
	signed, err := credential.dynamicClientAssertion(endpoint)
	require.NoError(t, err)
	claims := &jwtVerifier.RegisteredClaims{}
	token, err := jwtVerifier.ParseWithClaims(string(signed), claims, func(*jwtVerifier.Token) (interface{}, error) {
		return publicKey, nil
	}, jwtVerifier.WithValidMethods([]string{"RS256"}))
	require.NoError(t, err)
	require.True(t, token.Valid)
	require.Equal(t, serialized["kid"], token.Header["kid"])
	require.Equal(t, credential.DynamicClientId, claims.Subject)
	require.Equal(t, credential.DynamicClientId, claims.Issuer)
	require.Equal(t, jwtVerifier.ClaimStrings{endpoint}, claims.Audience)
	require.NotNil(t, claims.IssuedAt)
	require.NotNil(t, claims.ExpiresAt)
	require.WithinDuration(t, time.Now(), claims.IssuedAt.Time, 5*time.Second)
	require.WithinDuration(t, time.Now().Add(2*time.Minute), claims.ExpiresAt.Time, 5*time.Second)
	_, err = uuid.Parse(claims.ID)
	require.NoError(t, err)

	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	_, err = jwtVerifier.Parse(string(signed), func(*jwtVerifier.Token) (interface{}, error) {
		return &wrongKey.PublicKey, nil
	}, jwtVerifier.WithValidMethods([]string{"RS256"}))
	require.Error(t, err, "a different public key must not verify the assertion")
	parts := strings.Split(string(signed), ".")
	require.Len(t, parts, 3)
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"synthetic-tampered-client"}`))
	_, err = jwtVerifier.Parse(strings.Join(parts, "."), func(*jwtVerifier.Token) (interface{}, error) {
		return publicKey, nil
	}, jwtVerifier.WithValidMethods([]string{"RS256"}))
	require.Error(t, err, "modified claims must not verify")
}

func TestDynamicClientAssertionRejectsMissingCredentials(t *testing.T) {
	for _, credential := range []SourceCredential{
		{},
		{DynamicClientId: "synthetic-client"},
		{DynamicClientJWKS: []map[string]string{{"kty": "RSA"}}},
	} {
		signed, err := credential.dynamicClientAssertion("https://provider.example.invalid/token")
		require.Error(t, err)
		require.Empty(t, signed)
	}
}
