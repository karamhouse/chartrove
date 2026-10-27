package host

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestShouldQuitAfterDrainsWithoutBlockingNativeThread(t *testing.T) {
	// A request's final WebKit write needs the native thread after cancellation.
	// Model it by allowing close to finish only once ShouldQuit has returned.
	mainThreadAvailable := make(chan struct{})
	draining := make(chan struct{})
	quit := make(chan struct{}, 1)
	var closes atomic.Int32
	shouldQuit := ShouldQuitAfter(func() {
		closes.Add(1)
		close(draining)
		<-mainThreadAvailable
	}, func() { quit <- struct{}{} })
	returned := make(chan bool, 1)
	go func() { returned <- shouldQuit() }()
	select {
	case allowed := <-returned:
		if allowed {
			t.Fatal("termination allowed before backend drained")
		}
	case <-time.After(time.Second):
		close(mainThreadAvailable)
		t.Fatal("shutdown blocked the native thread needed by the response writer")
	}
	<-draining
	for range 10 {
		if shouldQuit() {
			t.Fatal("repeated quit allowed before draining completed")
		}
	}
	select {
	case <-quit:
		t.Fatal("quit retried before draining completed")
	default:
	}
	close(mainThreadAvailable)
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("native quit was not retried after backend drained")
	}
	if !shouldQuit() || closes.Load() != 1 {
		t.Fatal("drained shutdown must allow termination and close only once")
	}
}
