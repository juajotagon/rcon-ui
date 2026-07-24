package session

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/juajotagon/rcon-ui/internal/event"
)

// TestMarkBrokenSurvivesConcurrentReconnect is a regression test for a data
// race where markBroken could close an already-closed broken channel and
// panic the daemon.
//
// The bug: markBroken used to capture ch := s.broken under s.mu, release the
// lock, then call s.brokenOnce.Do(func() { close(ch) }) outside the lock.
// Do() reads whatever *sync.Once is live on s at the moment it runs, not the
// one that was live when ch was captured. supervise reassigns s.broken and
// resets s.brokenOnce together on every reconnect. So a second markBroken call
// could capture a stale (already-closed) channel, have a reconnect land in the
// gap before it calls Do, and pair that stale channel with the freshly-reset
// Once -- which then ran the close a second time.
//
// This test drives exactly that shape: two goroutines hammering markBroken
// (mirroring two commands in flight when the connection breaks) racing
// against a goroutine that resets broken/brokenClosed the way supervise does
// on reconnect. Under the old code this reliably panics (close of closed
// channel) within a handful of iterations; under the fix it must not, and
// `go test -race` must not report a data race on the broken/brokenClosed
// pair.
func TestMarkBrokenSurvivesConcurrentReconnect(t *testing.T) {
	hub := event.NewHub()
	mgr := &Manager{
		hub: hub,
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	s := &Session{
		mgr:     mgr,
		status:  StatusConnected,
		changed: make(chan struct{}),
		broken:  make(chan struct{}),
	}

	const duration = 100 * time.Millisecond
	stop := make(chan struct{})
	time.AfterFunc(duration, func() { close(stop) })

	var wg sync.WaitGroup

	// Mimics supervise's reconnect: reassigns broken and resets brokenClosed
	// together under s.mu, over and over, to keep the race window open.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			s.mu.Lock()
			s.broken = make(chan struct{})
			s.brokenClosed = false
			s.mu.Unlock()
		}
	}()

	// Two concurrent callers, per QA's repro shape: two commands in flight
	// both discover the connection is broken.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				s.markBroken(errors.New("boom"))
			}
		}()
	}

	wg.Wait()
}
