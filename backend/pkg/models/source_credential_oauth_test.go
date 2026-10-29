package models

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-sources/clients/factory"
	sourceModels "github.com/fastenhealth/fasten-sources/clients/models"
	definitionModels "github.com/fastenhealth/fasten-sources/definitions/models"
	sourcePkg "github.com/fastenhealth/fasten-sources/pkg"
	"github.com/fastenhealth/fasten-sources/pkg/models/catalog"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// Use the real credential setters without invoking remote provider discovery.
type syntheticOAuthCredential struct{ *SourceCredential }

func (*syntheticOAuthCredential) IsDynamicClient() bool { return false }

func TestProviderOAuthTokenCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name            string
		expired         bool
		responseRefresh string
		rejectRefresh   bool
	}{
		{name: "reuse valid token"},
		{name: "rotate refresh token", expired: true, responseRefresh: "synthetic-rotated-refresh"},
		{name: "preserve omitted refresh token", expired: true},
		{name: "reject invalid grant", expired: true, rejectRefresh: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var tokenRequests, resourceRequests atomic.Int32
			expiry := time.Now().Add(time.Hour).Unix()
			if tc.expired {
				expiry = time.Now().Add(-time.Hour).Unix()
			}
			credential := &syntheticOAuthCredential{&SourceCredential{
				ClientId: "synthetic-client", AccessToken: "synthetic-original-access",
				RefreshToken: "synthetic-original-refresh", ExpiresAt: expiry,
			}}
			wantAccess := credential.AccessToken
			if tc.expired {
				wantAccess = "synthetic-renewed-access"
			}

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/token":
					tokenRequests.Add(1)
					if r.Method != http.MethodPost || r.ParseForm() != nil ||
						r.PostForm.Get("grant_type") != "refresh_token" ||
						r.PostForm.Get("refresh_token") != "synthetic-original-refresh" {
						t.Error("unexpected token refresh request")
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if tc.rejectRefresh {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
						return
					}
					response := map[string]interface{}{
						"access_token": wantAccess, "token_type": "Bearer", "expires_in": 3600,
					}
					if tc.responseRefresh != "" {
						response["refresh_token"] = tc.responseRefresh
					}
					_ = json.NewEncoder(w).Encode(response)
				case "/fhir/Patient/synthetic":
					resourceRequests.Add(1)
					if r.Header.Get("Authorization") != "Bearer "+wantAccess {
						t.Error("FHIR request did not use the expected access token")
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					_, _ = io.WriteString(w, `{"resourceType":"Patient","id":"synthetic"}`)
				default:
					t.Errorf("unexpected synthetic endpoint: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			logger := logrus.New()
			logger.SetOutput(io.Discard)
			client, err := factory.GetSourceClientWithDefinition(
				sourcePkg.FastenLighthouseEnvSandbox, ctx, logger, credential,
				&definitionModels.LighthouseSourceDefinition{PatientAccessEndpoint: &catalog.PatientAccessEndpoint{
					Url: server.URL + "/fhir", TokenEndpoint: server.URL + "/token",
				}}, sourceModels.WithHttpClient(server.Client()),
			)
			if tc.rejectRefresh {
				require.Error(t, err)
				require.ErrorContains(t, err, "invalid_grant")
				require.GreaterOrEqual(t, tokenRequests.Load(), int32(1))
				require.Zero(t, resourceRequests.Load())
				require.Equal(t, "synthetic-original-access", credential.AccessToken)
				require.Equal(t, "synthetic-original-refresh", credential.RefreshToken)
				require.Equal(t, expiry, credential.ExpiresAt)
				return
			}
			require.NoError(t, err)
			var patient map[string]interface{}
			_, err = client.GetRequest("Patient/synthetic", &patient)
			require.NoError(t, err)
			require.Equal(t, "synthetic", patient["id"])
			require.Equal(t, int32(1), resourceRequests.Load())
			require.Equal(t, wantAccess, credential.AccessToken)
			wantRefresh := "synthetic-original-refresh"
			if tc.responseRefresh != "" {
				wantRefresh = tc.responseRefresh
			}
			require.Equal(t, wantRefresh, credential.RefreshToken)
			if tc.expired {
				require.Equal(t, int32(1), tokenRequests.Load(), "reuse the refreshed token for the FHIR request")
				require.Greater(t, credential.ExpiresAt, time.Now().Unix())
			} else {
				require.Zero(t, tokenRequests.Load())
				require.Equal(t, expiry, credential.ExpiresAt)
			}
		})
	}
}
