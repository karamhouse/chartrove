package host

import (
	"sync"
	"sync/atomic"
)

// ShouldQuitAfter drains the backend before allowing native termination. Wails
// calls ShouldQuit and OnShutdown on the native thread; blocking that thread in
// Close can deadlock requests whose final WebKit response needs the same thread.
// Reject the first quit, leave the native event loop running while Close drains,
// then retry quit once. Subsequent termination callbacks may safely return true.
func ShouldQuitAfter(closeBackend, quit func()) func() bool {
	var started sync.Once
	var drained atomic.Bool
	return func() bool {
		if drained.Load() {
			return true
		}
		started.Do(func() {
			go func() {
				closeBackend()
				drained.Store(true)
				quit()
			}()
		})
		return false
	}
}
