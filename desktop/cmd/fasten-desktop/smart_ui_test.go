package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyntheticSMARTModeRequiresFreshIsolatedData(t *testing.T) {
	fresh := filepath.Join(t.TempDir(), "new-data")
	if err := validateSMARTUITest(fresh, true, "", false); err != nil {
		t.Fatal(err)
	}
	for _, args := range []struct {
		dir    string
		plain  bool
		config string
		smoke  bool
	}{
		{"", true, "", false}, {fresh, false, "", false}, {fresh, true, "operator-config.json", false}, {fresh, true, "", true},
	} {
		if validateSMARTUITest(args.dir, args.plain, args.config, args.smoke) == nil {
			t.Fatal("unsafe synthetic mode accepted")
		}
	}
	existing := t.TempDir()
	if err := os.WriteFile(filepath.Join(existing, "fasten.db"), []byte("synthetic sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if validateSMARTUITest(existing, true, "", false) == nil {
		t.Fatal("existing data directory accepted")
	}
}
