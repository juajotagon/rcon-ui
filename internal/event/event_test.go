package event

import (
	"sync"
	"testing"
	"time"
)

func TestPublishAssignsMonotonicSeq(t *testing.T) {
	h := NewHub()

	var last uint64
	for i := range 10 {
		e := h.Publish(Event{ProfileID: "a", Line: "line"})
		if e.Seq <= last {
			t.Fatalf("event %d: seq %d did not increase from %d", i, e.Seq, last)
		}
		last = e.Seq
	}
}

// Seq is global rather than per-profile so it doubles as an SSE Last-Event-ID.
// Filtering a global sequence to one profile still yields a monotonic run,
// which is what the console needs to interleave sources.
func TestSeqStaysMonotonicWithinAProfile(t *testing.T) {
	h := NewHub()

	var seqs []uint64
	for range 5 {
		h.Publish(Event{ProfileID: "other"})
		seqs = append(seqs, h.Publish(Event{ProfileID: "mine"}).Seq)
	}

	for i := 1; i < len(seqs); i++ {
		if seqs[i] <= seqs[i-1] {
			t.Errorf("seq went backwards within a profile: %v", seqs)
		}
	}
}

func TestPublishSetsTimestamp(t *testing.T) {
	h := NewHub()
	if e := h.Publish(Event{ProfileID: "a"}); e.At.IsZero() {
		t.Error("At was not set")
	}

	explicit := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if e := h.Publish(Event{ProfileID: "a", At: explicit}); !e.At.Equal(explicit) {
		t.Errorf("At = %v, want the caller's value %v", e.At, explicit)
	}
}

func TestSubscribeReceivesEvents(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("")
	defer unsub()

	h.Publish(Event{ProfileID: "a", Line: "hello"})

	select {
	case e := <-ch:
		if e.Line != "hello" {
			t.Errorf("line = %q", e.Line)
		}
	case <-time.After(time.Second):
		t.Fatal("no event received")
	}
}

func TestSubscribeFiltersByProfile(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("wanted")
	defer unsub()

	h.Publish(Event{ProfileID: "unwanted", Line: "no"})
	h.Publish(Event{ProfileID: "wanted", Line: "yes"})

	select {
	case e := <-ch:
		if e.Line != "yes" {
			t.Errorf("received an event for the wrong profile: %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("no event received")
	}
}

// A stalled browser tab must not be able to block the RCON session that is
// producing events.
func TestPublishDoesNotBlockOnSlowSubscriber(t *testing.T) {
	h := NewHub()
	_, unsub := h.Subscribe("") // never drained
	defer unsub()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range subscriberBuffer * 3 {
			h.Publish(Event{ProfileID: "a", Line: "flood"})
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked on a subscriber that was not reading")
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("")
	unsub()

	if _, open := <-ch; open {
		t.Error("channel still open after unsubscribe")
	}
	unsub() // must not panic on a second call
}

func TestHistoryReturnsEventsAfterSeq(t *testing.T) {
	h := NewHub()

	first := h.Publish(Event{ProfileID: "a", Line: "one"})
	h.Publish(Event{ProfileID: "a", Line: "two"})
	h.Publish(Event{ProfileID: "b", Line: "other"})

	all := h.History("", 0)
	if len(all) != 3 {
		t.Errorf("got %d events, want 3", len(all))
	}

	after := h.History("", first.Seq)
	if len(after) != 2 {
		t.Errorf("after seq %d: got %d events, want 2", first.Seq, len(after))
	}

	filtered := h.History("a", 0)
	if len(filtered) != 2 {
		t.Errorf("profile a: got %d events, want 2", len(filtered))
	}
}

func TestHistoryIsBounded(t *testing.T) {
	h := NewHub()
	h.historyLimit = 10

	for range 50 {
		h.Publish(Event{ProfileID: "a"})
	}

	if got := len(h.History("", 0)); got != 10 {
		t.Errorf("retained %d events, want the 10-event limit", got)
	}
}

// Regression: unsubscribing while a Publish was mid-fan-out closed the channel
// under the sender, panicking the daemon with "send on closed channel". A UI
// closing its event stream at the wrong instant was enough to trigger it.
func TestUnsubscribeDuringPublishDoesNotPanic(t *testing.T) {
	h := NewHub()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				h.Publish(Event{ProfileID: "a", Line: "x"})
			}
		}
	}()

	// Churn subscriptions against the publisher.
	for range 200 {
		_, unsub := h.Subscribe("")
		unsub()
	}

	close(stop)
	wg.Wait()
}

// Concurrent publishers must still hand out strictly increasing Seq, because
// consumers discard anything at or below what they have already seen.
func TestConcurrentPublishSeqIsOrdered(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("")
	defer unsub()

	const n = 100
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Publish(Event{ProfileID: "a"})
		}()
	}
	wg.Wait()

	var last uint64
	for range n {
		select {
		case e := <-ch:
			if e.Seq <= last {
				t.Fatalf("events delivered out of order: %d after %d", e.Seq, last)
			}
			last = e.Seq
		default:
			return // buffer exhausted; what we did read was ordered
		}
	}
}

func TestConcurrentPublishAndSubscribe(t *testing.T) {
	h := NewHub()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unsub := h.Subscribe("")
			defer unsub()
			for range 50 {
				h.Publish(Event{ProfileID: "a"})
				select {
				case <-ch:
				default:
				}
			}
		}()
	}
	wg.Wait()

	// 8 goroutines x 50 events; the exact count matters less than that the
	// counter never repeated, which the race detector and this check cover.
	if seq := h.Publish(Event{}).Seq; seq != 401 {
		t.Errorf("final seq = %d, want 401 (8*50 + 1)", seq)
	}
}
