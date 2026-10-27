package event_bus

import (
	"io"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/sirupsen/logrus"
)

func TestCancelledListenerCannotBlockBroadcastOrRemoval(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	bus := NewEventBusServer(logger)
	done := make(chan struct{})
	listener := &EventBusListener{UserID: "synthetic-room", ResponseChan: make(chan string), Done: done}
	bus.AddListener(listener)
	// An unbuffered publish returns once the bus has received the message; the
	// bus then tries to send to a client that deliberately never reads it.
	if err := bus.PublishMessage(models.NewEventKeepAlive("synthetic-room")); err != nil {
		t.Fatal(err)
	}
	close(done)
	removed := make(chan struct{})
	go func() { bus.RemoveListener(listener); close(removed) }()
	select {
	case <-removed:
	case <-time.After(time.Second):
		t.Fatal("cancelled stream deadlocked event bus removal")
	}
}
