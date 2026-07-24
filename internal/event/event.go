// Package event defines the single envelope every server-to-client message
// uses, and the hub that fans those messages out to connected UIs.
//
// The shape is deliberately more general than v1 needs. Today the only
// producer is RCON command execution, but live log streaming (Kubernetes pod
// logs, file tail, docker logs) is a planned addition, and retrofitting it
// would otherwise mean reworking both the transport and the console component.
// Building the envelope generic now costs a slightly wider struct; building it
// as {command, response} pairs would cost a rewrite.
package event

import (
	"context"
	"sync"
	"time"
)

// Source identifies where an event came from.
const (
	// SourceRCON is command execution over RCON: the only producer in v1.
	SourceRCON = "rcon"

	// SourceSystem is rcon-ui itself reporting connection state.
	SourceSystem = "system"
)

// Stream identifies what kind of line this is, within a source.
const (
	StreamCommand  = "command"  // a command the user sent
	StreamResponse = "response" // a server's reply to that command
	StreamStatus   = "status"   // a connection state change
	StreamError    = "error"    // a failure worth showing in the console
)

// Event is one line destined for a console pane.
//
// A command and its reply are two Events sharing a ProfileID, never one paired
// object. That distinction is the whole point of the envelope: a log line from
// a future LogSource has no "command" half, so any shape that couples the two
// cannot represent it.
type Event struct {
	// Seq orders events. It is assigned by the Hub and is globally monotonic,
	// which makes it usable directly as an SSE Last-Event-ID for resuming a
	// dropped stream. Because a global sequence stays monotonic when filtered
	// to one profile, this also gives the per-profile ordering the console
	// needs to interleave several sources.
	Seq uint64 `json:"seq"`

	ProfileID string    `json:"profileId"`
	Source    string    `json:"source"`
	Stream    string    `json:"stream"`
	Line      string    `json:"line"`
	At        time.Time `json:"at"`
}

// LogSource streams lines from somewhere other than RCON.
//
// Declared with no implementations in v1. It exists so the shape is fixed
// while the hub and console are being written, rather than being invented
// later against code that has already hardened around a single producer.
//
// Note that any implementation necessarily breaks the "runs anywhere" property
// RCON has: reading logs needs access to the game host or cluster, whereas
// RCON needs only a reachable port. Log sources are therefore an optional
// per-profile addition, never a requirement for a server to work.
type LogSource interface {
	// Stream sends events until ctx is cancelled or the source ends.
	Stream(ctx context.Context, out chan<- Event) error
	Close() error
}

// subscriberBuffer is how many events a slow consumer may fall behind before
// its events start being dropped. Generous enough to absorb a burst of
// multi-packet output without letting one stalled browser tab pin memory.
const subscriberBuffer = 256

// defaultHistory is how many recent events the Hub retains for replay. Sized so
// a console has content immediately on connect, and so a brief network blip
// resumes without a visible gap.
const defaultHistory = 500

// Hub fans events out to subscribers and retains a bounded replay buffer.
//
// Safe for concurrent use.
type Hub struct {
	mu           sync.RWMutex
	seq          uint64
	subs         map[uint64]*subscriber
	nextSubID    uint64
	history      []Event
	historyLimit int
}

type subscriber struct {
	ch chan Event

	// profileID filters this subscriber to one server. Empty means all.
	profileID string
}

func NewHub() *Hub {
	return &Hub{
		subs:         make(map[uint64]*subscriber),
		historyLimit: defaultHistory,
	}
}

// Publish stamps e with the next sequence number and current time, delivers it
// to matching subscribers, and records it for replay. It returns the stamped
// event.
//
// Publish never blocks: a subscriber whose buffer is full misses the event
// rather than stalling the RCON session that produced it. Clients detect the
// gap from the jump in Seq and can re-fetch history.
//
// The lock is deliberately held across the fan-out, which is safe because every
// send is non-blocking and therefore bounded. Releasing it first would be wrong
// twice over:
//
//   - A concurrent unsubscribe could close a subscriber's channel between the
//     unlock and the send, panicking the whole daemon with "send on closed
//     channel". Closing requires the write lock, so holding it here makes that
//     impossible.
//   - Two concurrent Publish calls could deliver out of Seq order. Consumers
//     treat Seq as monotonic -- the SSE handler discards anything at or below
//     what it has already sent -- so a late-arriving lower Seq would be dropped
//     rather than reordered, silently losing an event.
func (h *Hub) Publish(e Event) Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.seq++
	e.Seq = h.seq
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}

	h.history = append(h.history, e)
	if len(h.history) > h.historyLimit {
		// Copy rather than reslice: reslicing keeps the whole backing array
		// alive, so trimmed events would never be collected.
		h.history = append([]Event(nil), h.history[len(h.history)-h.historyLimit:]...)
	}

	for _, s := range h.subs {
		if s.profileID != "" && s.profileID != e.ProfileID {
			continue
		}
		select {
		case s.ch <- e:
		default: // slow consumer; drop rather than block the producer
		}
	}
	return e
}

// Subscribe returns a channel of events and a function to release it. Passing
// an empty profileID subscribes to every profile.
//
// The returned cancel function must be called, or the subscriber leaks.
func (h *Hub) Subscribe(profileID string) (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.nextSubID++
	id := h.nextSubID
	s := &subscriber{ch: make(chan Event, subscriberBuffer), profileID: profileID}
	h.subs[id] = s

	return s.ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(s.ch)
		}
	}
}

// History returns retained events with Seq greater than after, optionally
// filtered to one profile. Passing after=0 returns everything retained.
//
// This backs both the initial console fill and SSE resumption via
// Last-Event-ID. Events older than the retention window are simply absent --
// callers must treat history as best-effort, not as durable storage.
func (h *Hub) History(profileID string, after uint64) []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]Event, 0, len(h.history))
	for _, e := range h.history {
		if e.Seq <= after {
			continue
		}
		if profileID != "" && e.ProfileID != profileID {
			continue
		}
		out = append(out, e)
	}
	return out
}
