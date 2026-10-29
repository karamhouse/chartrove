package ips_pdf

import (
	"bytes"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/models/database"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/utils/ips"
	"github.com/fastenhealth/gofhir-models/fhir401"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestGeneratePDFOutsideSourceTree(t *testing.T) {
	// Installed binaries must not depend on a source checkout or their launch directory.
	t.Chdir(t.TempDir())
	id, system := "synthetic-summary", "https://example.invalid/chartrove-test"
	data := &ips.InternationalPatientSummaryExportData{
		GenerationDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Composition: &fhir401.Composition{
			Identifier: &fhir401.Identifier{Value: &id, System: &system},
		},
		Patient: &database.FhirPatient{
			Name: datatypes.JSON(`["Synthetic Test Patient"]`),
		},
	}

	pdf, err := NewPDFRenderer().Render(data)
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(pdf, []byte("%PDF-")), "expected a PDF document")
	require.True(t, bytes.Contains(pdf, []byte("/Subtype /Image")), "the preserved logo must be embedded")
	require.False(t, bytes.Contains(pdf, []byte("could not load image")), "the PDF must not contain an image error")
	require.True(t, bytes.HasSuffix(bytes.TrimSpace(pdf), []byte("%%EOF")), "expected a complete PDF")
}
