package platform

import (
	"runtime"
	"sync"

	"citadel-seeder/internal/logger"
)

// acquireThreadResource keeps acquisition and cleanup on the same dedicated OS thread.
func acquireThreadResource(acquire func() error, release func() error) (ReleaseFunc, error) {
	started := make(chan error, 1)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		// Exiting while locked terminates the thread instead of returning thread state to Go's pool.
		runtime.LockOSThread()
		defer close(done)
		if err := acquire(); err != nil {
			started <- err
			return
		}
		started <- nil
		<-stop
		if err := release(); err != nil {
			logger.Warn("Could not release OS thread resource", "error", err)
		}
	}()
	if err := <-started; err != nil {
		<-done
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { close(stop); <-done }) }, nil
}
