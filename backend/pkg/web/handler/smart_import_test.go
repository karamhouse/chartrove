package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/pkg"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/database"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/event_bus"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type smartImportHarness struct {
	router    *gin.Engine
	repo      database.DatabaseRepository
	ctx       context.Context
	uploadDir string
}

func newSMARTImportHarness(t *testing.T) smartImportHarness {
	t.Helper()
	cfg, err := config.Create()
	require.NoError(t, err)
	cfg.Set("database.type", "sqlite")
	cfg.Set("database.location", filepath.Join(t.TempDir(), "synthetic.db"))
	cfg.Set("database.encryption.enabled", false)
	cfg.Set("database.validation_mode", false)
	cfg.Set("log.level", "WARN")
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	entry := logrus.NewEntry(logger)
	bus := event_bus.NewNoopEventBusServer()
	repo, err := database.NewRepository(cfg, entry, bus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	require.NoError(t, repo.CreateUser(context.Background(), &models.User{Username: "synthetic-smart", Password: "synthetic-password"}))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(pkg.ContextKeyTypeLogger, entry)
		c.Set(pkg.ContextKeyTypeDatabase, repo)
		c.Set(pkg.ContextKeyTypeConfig, cfg)
		c.Set(pkg.ContextKeyTypeEventBusServer, bus)
		c.Set(pkg.ContextKeyTypeAuthUsername, "synthetic-smart")
	})
	router.POST("/api/secure/source/manual", CreateManualSource)
	uploadDir := t.TempDir()
	t.Setenv("TMPDIR", uploadDir)
	return smartImportHarness{router: router, repo: repo, ctx: context.WithValue(context.Background(), pkg.ContextKeyTypeAuthUsername, "synthetic-smart"), uploadDir: uploadDir}
}

func (h smartImportHarness) post(t *testing.T, origin *string, bundle string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, h.request(t, origin, bundle))
	files, err := os.ReadDir(h.uploadDir)
	require.NoError(t, err)
	require.Empty(t, files, "upload temporary files must be removed on every return")
	return w
}

func (h smartImportHarness) request(t *testing.T, origin *string, bundle string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if origin != nil {
		require.NoError(t, writer.WriteField("smart_origin", *origin))
	}
	part, err := writer.CreateFormFile("file", "synthetic-smart.json")
	require.NoError(t, err)
	_, err = io.WriteString(part, bundle)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/secure/source/manual", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func smartPatientBundle(id, family string) string {
	resource, _ := json.Marshal(map[string]interface{}{"resourceType": "Patient", "id": id, "name": []map[string]string{{"family": family}}})
	return `{"resourceType":"Bundle","type":"collection","entry":[{"resource":` + string(resource) + `}]}`
}

type smartImportResponse struct {
	Success         bool                    `json:"success"`
	AlreadyImported bool                    `json:"already_imported"`
	Source          models.SourceCredential `json:"source"`
}

func readSMARTImportResponse(t *testing.T, w *httptest.ResponseRecorder) smartImportResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var response smartImportResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response
}

// Dropping provenance or dedup would merge provider identities or overwrite a
// snapshot when the user confirms a preview more than once.
func TestSMARTImportProvenanceAndCompletedSnapshotDedup(t *testing.T) {
	h := newSMARTImportHarness(t)
	originA := `{"fhir_base_url":"https://provider-a.example/fhir///","display":"Provider A"}`
	first := readSMARTImportResponse(t, h.post(t, &originA, smartPatientBundle("erXu", "Original")))
	require.False(t, first.AlreadyImported)
	require.NotEqual(t, uuid.Nil, first.Source.EndpointID)
	require.Equal(t, "manual", string(first.Source.PlatformType))
	require.Equal(t, "erXu", first.Source.Patient, "direct snapshots retain raw FHIR IDs")
	require.Contains(t, first.Source.Display, "Direct SMART snapshot")
	require.Contains(t, first.Source.Display, "Provider A")
	require.Contains(t, first.Source.Display, "https://provider-a.example/fhir")

	normalizedA := `{"fhir_base_url":"https://provider-a.example/fhir","display":"Renamed Provider A"}`
	repeat := readSMARTImportResponse(t, h.post(t, &normalizedA, smartPatientBundle("erXu", "Must not overwrite")))
	require.True(t, repeat.AlreadyImported)
	require.Equal(t, first.Source.ID, repeat.Source.ID)
	require.Equal(t, first.Source.Display, repeat.Source.Display)
	resources, err := h.repo.ListResources(h.ctx, models.ListResourceQueryOptions{SourceID: first.Source.ID.String(), SourceResourceType: "Patient"})
	require.NoError(t, err)
	require.Len(t, resources, 1)
	require.Contains(t, string(resources[0].ResourceRaw), "Original")
	require.NotContains(t, string(resources[0].ResourceRaw), "Must not overwrite")

	originB := `{"fhir_base_url":"https://provider-b.example/fhir","display":"Provider B"}`
	second := readSMARTImportResponse(t, h.post(t, &originB, smartPatientBundle("erXu", "Other provider")))
	require.NotEqual(t, first.Source.ID, second.Source.ID)
	require.NotEqual(t, first.Source.EndpointID, second.Source.EndpointID)

	plain := readSMARTImportResponse(t, h.post(t, nil, smartPatientBundle("erXu", "Plain manual")))
	require.Equal(t, uuid.Nil, plain.Source.EndpointID)
	require.Empty(t, plain.Source.Display)
	require.Equal(t, "rXu", plain.Source.Patient, "legacy importer behavior stays unchanged")
	require.False(t, plain.AlreadyImported)
	require.NotEqual(t, first.Source.ID, plain.Source.ID)
	plainRepeat := readSMARTImportResponse(t, h.post(t, nil, smartPatientBundle("erXu", "Plain manual update")))
	require.False(t, plainRepeat.AlreadyImported)
	require.Equal(t, plain.Source.ID, plainRepeat.Source.ID)
	plainResources, err := h.repo.ListResources(h.ctx, models.ListResourceQueryOptions{SourceID: plain.Source.ID.String(), SourceResourceType: "Patient"})
	require.NoError(t, err)
	require.Len(t, plainResources, 1)
	require.Contains(t, string(plainResources[0].ResourceRaw), "Plain manual update")

	cutsetCollision := readSMARTImportResponse(t, h.post(t, &originA, smartPatientBundle("rXu", "Different patient")))
	require.NotEqual(t, first.Source.ID, cutsetCollision.Source.ID)
	require.Equal(t, "rXu", cutsetCollision.Source.Patient)
}

// Metadata must be rejected before creating an unprovenanced manual source.
func TestSMARTImportRejectsInvalidOriginAndNonPatientSnapshots(t *testing.T) {
	h := newSMARTImportHarness(t)
	valid := `{"fhir_base_url":"https://provider.example/fhir","display":"Provider"}`
	cases := []struct{ name, origin, bundle string }{
		{"empty metadata", "", smartPatientBundle("synthetic-1", "Synthetic")},
		{"insecure", `{"fhir_base_url":"http://provider.example/fhir","display":"Provider"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"relative", `{"fhir_base_url":"/fhir","display":"Provider"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"credentials", `{"fhir_base_url":"https://user@provider.example/fhir","display":"Provider"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"query", `{"fhir_base_url":"https://provider.example/fhir?token=public-test","display":"Provider"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"fragment", `{"fhir_base_url":"https://provider.example/fhir#fragment","display":"Provider"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"empty query", `{"fhir_base_url":"https://provider.example/fhir?","display":"Provider"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"empty fragment", `{"fhir_base_url":"https://provider.example/fhir#","display":"Provider"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"unknown field", `{"fhir_base_url":"https://provider.example/fhir","display":"Provider","access_token":"public-test"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"missing display", `{"fhir_base_url":"https://provider.example/fhir"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"long display", `{"fhir_base_url":"https://provider.example/fhir","display":"` + strings.Repeat("x", 101) + `"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"long url", `{"fhir_base_url":"https://provider.example/` + strings.Repeat("x", 2048) + `","display":"Provider"}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"oversized metadata", strings.Repeat(" ", 4097) + valid, smartPatientBundle("synthetic-1", "Synthetic")},
		{"null", "null", smartPatientBundle("synthetic-1", "Synthetic")},
		{"trailing json", valid + `{}`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"malformed metadata", `{`, smartPatientBundle("synthetic-1", "Synthetic")},
		{"not bundle", valid, `{"resourceType":"Patient","id":"synthetic-1"}`},
		{"not patient", valid, `{"resourceType":"Bundle","entry":[{"resource":{"resourceType":"Observation","id":"synthetic-1"}}]}`},
		{"two patients", valid, `{"resourceType":"Bundle","entry":[{"resource":{"resourceType":"Patient","id":"synthetic-1"}},{"resource":{"resourceType":"Patient","id":"synthetic-2"}}]}`},
		{"extra resource", valid, `{"resourceType":"Bundle","entry":[{"resource":{"resourceType":"Patient","id":"synthetic-1"}},{"resource":{"resourceType":"Observation","id":"synthetic-2"}}]}`},
		{"missing id", valid, `{"resourceType":"Bundle","entry":[{"resource":{"resourceType":"Patient"}}]}`},
		{"invalid id", valid, smartPatientBundle("Patient/synthetic-1", "Synthetic")},
		{"invalid utf8", valid, strings.Replace(smartPatientBundle("synthetic-1", "Synthetic"), "Synthetic", string([]byte{0xff}), 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := h.post(t, &tc.origin, tc.bundle)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
	sources, err := h.repo.GetSources(h.ctx)
	require.NoError(t, err)
	for _, source := range sources {
		require.NotEqual(t, "manual", string(source.PlatformType))
	}
}

// Source creation precedes resource persistence. A failed write must leave a
// retryable source rather than falsely report a completed snapshot next time.
func TestSMARTImportRetriesMissingPatientOnExistingSource(t *testing.T) {
	h := newSMARTImportHarness(t)
	origin := `{"fhir_base_url":"https://retry.example/fhir","display":"Retry Provider"}`
	client := h.repo.(*database.GormRepository).GormClient
	require.NoError(t, client.Exec(`CREATE TRIGGER synthetic_fail_patient BEFORE INSERT ON fhir_patient BEGIN SELECT RAISE(FAIL, 'synthetic write failure'); END`).Error)
	w := h.post(t, &origin, smartPatientBundle("synthetic-retry", "Synthetic"))
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	sources, err := h.repo.GetSources(h.ctx)
	require.NoError(t, err)
	var failed models.SourceCredential
	for _, source := range sources {
		if string(source.PlatformType) == "manual" {
			failed = source
		}
	}
	require.NotEqual(t, uuid.Nil, failed.ID)
	require.NoError(t, client.Exec(`DROP TRIGGER synthetic_fail_patient`).Error)
	retry := readSMARTImportResponse(t, h.post(t, &origin, smartPatientBundle("synthetic-retry", "Synthetic")))
	require.False(t, retry.AlreadyImported)
	require.Equal(t, failed.ID, retry.Source.ID)
	resources, err := h.repo.ListResources(h.ctx, models.ListResourceQueryOptions{SourceID: retry.Source.ID.String(), SourceResourceType: "Patient"})
	require.NoError(t, err)
	require.Len(t, resources, 1)
	completed := readSMARTImportResponse(t, h.post(t, &origin, smartPatientBundle("synthetic-retry", "Later")))
	require.True(t, completed.AlreadyImported)
}

func TestSMARTImportConcurrentConfirmationsKeepOneSnapshot(t *testing.T) {
	h := newSMARTImportHarness(t)
	origin := `{"fhir_base_url":"https://concurrent.example/fhir","display":"Concurrent Provider"}`
	requests := []*http.Request{
		h.request(t, &origin, smartPatientBundle("synthetic-concurrent", "First contender")),
		h.request(t, &origin, smartPatientBundle("synthetic-concurrent", "Second contender")),
	}
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, len(requests))
	for _, request := range requests {
		go func(request *http.Request) {
			<-start
			w := httptest.NewRecorder()
			h.router.ServeHTTP(w, request)
			responses <- w
		}(request)
	}
	close(start)
	first := readSMARTImportResponse(t, <-responses)
	second := readSMARTImportResponse(t, <-responses)
	require.Equal(t, first.Source.ID, second.Source.ID)
	require.NotEqual(t, first.AlreadyImported, second.AlreadyImported, "one request imports and the other observes completion")
	resources, err := h.repo.ListResources(h.ctx, models.ListResourceQueryOptions{SourceID: first.Source.ID.String(), SourceResourceType: "Patient"})
	require.NoError(t, err)
	require.Len(t, resources, 1)
	files, err := os.ReadDir(h.uploadDir)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestSMARTImportCannotBeOverwrittenByLaterPlainManualImport(t *testing.T) {
	h := newSMARTImportHarness(t)
	origin := `{"fhir_base_url":"https://direct-first.example/fhir","display":"Direct First Provider"}`
	direct := readSMARTImportResponse(t, h.post(t, &origin, smartPatientBundle("synthetic-same-id", "Direct original")))
	plain := readSMARTImportResponse(t, h.post(t, nil, smartPatientBundle("synthetic-same-id", "Plain manual")))
	require.NotEqual(t, direct.Source.ID, plain.Source.ID)
	require.Equal(t, uuid.Nil, plain.Source.EndpointID)
	resources, err := h.repo.ListResources(h.ctx, models.ListResourceQueryOptions{SourceID: direct.Source.ID.String(), SourceResourceType: "Patient"})
	require.NoError(t, err)
	require.Len(t, resources, 1)
	require.Contains(t, string(resources[0].ResourceRaw), "Direct original")
	require.NotContains(t, string(resources[0].ResourceRaw), "Plain manual")
}

func TestSMARTImportRejectsSnapshotLargerThanThreeMiB(t *testing.T) {
	h := newSMARTImportHarness(t)
	origin := `{"fhir_base_url":"https://bounded.example/fhir","display":"Bounded Provider"}`
	oversized := smartPatientBundle("synthetic-oversized", strings.Repeat("x", 3*1024*1024))
	w := h.post(t, &origin, oversized)
	require.Equal(t, http.StatusBadRequest, w.Code, "oversized direct snapshots must be rejected")
	sources, err := h.repo.GetSources(h.ctx)
	require.NoError(t, err)
	for _, source := range sources {
		require.NotEqual(t, "manual", string(source.PlatformType), "size validation must precede source creation")
	}
}
