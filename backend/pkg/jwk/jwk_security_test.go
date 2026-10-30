package jwk

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJWKDeserializeRejectsNonPrivateRSAKeys(t *testing.T) {
	key, err := JWKGenerate()
	require.NoError(t, err)
	serialized, err := JWKSerialize(key)
	require.NoError(t, err)
	for _, tt := range []struct {
		name string
		key  map[string]string
	}{
		{"public RSA", map[string]string{"kty": "RSA", "n": serialized["n"], "e": serialized["e"]}},
		{"symmetric", map[string]string{"kty": "oct", "k": "c3ludGhldGljLWtleQ"}},
		{"missing type", map[string]string{}},
		{"invalid encoding", map[string]string{"kty": "RSA", "n": "!", "e": "AQAB", "d": "!"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				key, err := JWKDeserialize(tt.key)
				if err == nil || key != nil {
					t.Error("invalid private-key input must return an error and no key")
				}
			})
		})
	}
}

func TestJWKSerializeEscapesCustomFieldNames(t *testing.T) {
	key, err := JWKGenerate()
	require.NoError(t, err)
	field := `x": "injected", "extra`
	require.NoError(t, key.Set(field, "synthetic-value"))
	serialized, err := JWKSerialize(key)
	require.NoError(t, err)
	// Avoid printing private key material if serialization regresses.
	if _, injected := serialized["extra"]; injected {
		t.Fatal("custom JWK field name created an unintended field")
	}
	if value, found := serialized[field]; !found || value != "synthetic-value" {
		t.Fatal("custom JWK field name did not round trip literally")
	}
}
