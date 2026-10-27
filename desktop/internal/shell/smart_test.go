package shell

import (
	"context"
	"testing"
)

func TestSMARTServiceRejectsCallsWithoutMainWindow(t *testing.T) {
	// A rejected window must never reach the manager (nil deliberately proves it).
	service := NewSMARTService(nil)
	ctx := context.Background()
	if _, err := service.Providers(ctx); err == nil {
		t.Fatal("untrusted provider listing")
	}
	if _, err := service.Connect(ctx, "epic-test"); err == nil {
		t.Fatal("untrusted browser launch")
	}
	if _, err := service.ReadPatient(ctx, "connection"); err == nil {
		t.Fatal("untrusted Patient read")
	}
	if err := service.Cancel(ctx, "epic-test"); err == nil {
		t.Fatal("untrusted cancellation")
	}
	if err := service.Disconnect(ctx, "connection"); err == nil {
		t.Fatal("untrusted disconnect")
	}
}
