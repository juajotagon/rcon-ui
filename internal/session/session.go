// Package session keeps live RCON connections and reports their state.
//
// One session per server profile, each supervised by a goroutine that connects,
// notices failures and reconnects with exponential backoff. Every state change
// is published to the event hub, so the UI reflects reality without polling.
package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/juajotagon/rcon-ui/internal/event"
	"github.com/juajotagon/rcon-ui/internal/rcon"
	"github.com/juajotagon/rcon-ui/internal/store"
)

// Status is a session's connection state.
type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"

	// StatusAuthFailed is terminal. The supervisor stops retrying, because
	// re-submitting a password the server has already rejected cannot succeed
	// and can trip ban logic on some servers. Clearing it requires the user to
	// fix the password, which restarts the session.
	StatusAuthFailed Status = "auth_failed"
)

var (
	// ErrNotConnected means a command was issued while no connection was
	// available and none became available in time.
	ErrNotConnected = errors.New("session: not connected")

	// ErrAuthFailed mirrors the terminal auth state for callers that only see
	// the session layer.
	ErrAuthFailed = errors.New("session: authentication failed")
)

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
)

// Manager owns every session.
type Manager struct {
	store *store.Store
	hub   *event.Hub
	log   *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc

	mu   sync.Mutex
	sess map[string]*Session
	wg   sync.WaitGroup
}

func NewManager(st *store.Store, hub *event.Hub, log *slog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		store:  st,
		hub:    hub,
		log:    log,
		ctx:    ctx,
		cancel: cancel,
		sess:   make(map[string]*Session),
	}
}

// Connect starts (or restarts) the session for a profile. It returns as soon as
// the supervisor is running; use Status or the event stream to observe the
// outcome.
func (m *Manager) Connect(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.sess[profileID]; ok {
		// Restarting a session is how a corrected password clears the
		// terminal auth_failed state.
		existing.stop()
		delete(m.sess, profileID)
	}

	s := &Session{
		mgr:       m,
		profileID: profileID,
		status:    StatusDisconnected,
		changed:   make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(m.ctx)
	s.cancel = cancel
	m.sess[profileID] = s

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		s.supervise(ctx)
	}()
	return nil
}

// Disconnect stops a session and closes its connection.
func (m *Manager) Disconnect(profileID string) {
	m.mu.Lock()
	s, ok := m.sess[profileID]
	if ok {
		delete(m.sess, profileID)
	}
	m.mu.Unlock()

	if ok {
		s.stop()
		s.setStatus(StatusDisconnected, "")
	}
}

// Execute runs a command, connecting on demand if the profile has no session.
//
// Both the command and its response are published as separate events, so the
// console is fed by the same stream a future log source would use rather than
// by this function's return value.
func (m *Manager) Execute(ctx context.Context, profileID, cmd string) (string, error) {
	m.mu.Lock()
	s, ok := m.sess[profileID]
	m.mu.Unlock()

	if !ok {
		if err := m.Connect(profileID); err != nil {
			return "", err
		}
		m.mu.Lock()
		s = m.sess[profileID]
		m.mu.Unlock()
	}

	client, err := s.waitReady(ctx)
	if err != nil {
		return "", err
	}

	m.hub.Publish(event.Event{
		ProfileID: profileID,
		Source:    event.SourceRCON,
		Stream:    event.StreamCommand,
		Line:      cmd,
	})

	resp, err := client.Execute(ctx, cmd)
	if err != nil {
		s.markBroken(err)
		m.hub.Publish(event.Event{
			ProfileID: profileID,
			Source:    event.SourceRCON,
			Stream:    event.StreamError,
			Line:      err.Error(),
		})
		return "", err
	}

	m.hub.Publish(event.Event{
		ProfileID: profileID,
		Source:    event.SourceRCON,
		Stream:    event.StreamResponse,
		Line:      resp,
	})

	// History is a convenience, not part of the command's contract: failing to
	// record it must not fail the command the user just ran.
	if err := m.store.AddHistory(ctx, profileID, cmd); err != nil {
		m.log.Warn("could not record command history", "profile", profileID, "error", err)
	}
	return resp, nil
}

// Status reports a profile's state. Profiles with no session read as
// disconnected, which is what the UI should show for a server nobody has
// opened yet.
func (m *Manager) Status(profileID string) Status {
	m.mu.Lock()
	s, ok := m.sess[profileID]
	m.mu.Unlock()

	if !ok {
		return StatusDisconnected
	}
	return s.Status()
}

// Statuses snapshots every known session.
func (m *Manager) Statuses() map[string]Status {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sess))
	for _, s := range m.sess {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()

	out := make(map[string]Status, len(sessions))
	for _, s := range sessions {
		out[s.profileID] = s.Status()
	}
	return out
}

// Close stops every session and waits for the supervisors to exit.
func (m *Manager) Close() {
	m.cancel()

	m.mu.Lock()
	for id, s := range m.sess {
		s.stop()
		delete(m.sess, id)
	}
	m.mu.Unlock()

	m.wg.Wait()
}

// Session is one supervised connection.
type Session struct {
	mgr       *Manager
	profileID string
	cancel    context.CancelFunc

	mu      sync.Mutex
	client  rcon.Client
	status  Status
	lastErr string

	// changed is closed and replaced on every status change, so waiters can
	// block on a state transition without polling.
	changed chan struct{}

	// broken is signalled by Execute when a command fails, waking the
	// supervisor to reconnect rather than waiting for the next command.
	brokenOnce sync.Once
	broken     chan struct{}
}

func (s *Session) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Session) setStatus(st Status, errMsg string) {
	s.mu.Lock()
	if s.status == st && s.lastErr == errMsg {
		s.mu.Unlock()
		return
	}
	s.status = st
	s.lastErr = errMsg

	close(s.changed)
	s.changed = make(chan struct{})
	s.mu.Unlock()

	line := string(st)
	if errMsg != "" {
		line = fmt.Sprintf("%s: %s", st, errMsg)
	}
	s.mgr.hub.Publish(event.Event{
		ProfileID: s.profileID,
		Source:    event.SourceSystem,
		Stream:    event.StreamStatus,
		Line:      line,
	})
}

func (s *Session) stop() {
	s.cancel()

	s.mu.Lock()
	client := s.client
	s.client = nil
	s.mu.Unlock()

	if client != nil {
		_ = client.Close()
	}
}

// markBroken records that the connection failed, so the supervisor reconnects.
func (s *Session) markBroken(err error) {
	s.mu.Lock()
	client := s.client
	s.client = nil
	ch := s.broken
	s.mu.Unlock()

	if client != nil {
		_ = client.Close()
	}
	if ch != nil {
		s.brokenOnce.Do(func() { close(ch) })
	}
	s.setStatus(StatusDisconnected, err.Error())
}

// waitReady returns the live client, waiting for a connection if one is being
// established. It fails fast on the terminal auth state rather than making the
// caller wait out a timeout that can never succeed.
func (s *Session) waitReady(ctx context.Context) (rcon.Client, error) {
	for {
		s.mu.Lock()
		client, status, lastErr, changed := s.client, s.status, s.lastErr, s.changed
		s.mu.Unlock()

		switch {
		case client != nil && status == StatusConnected:
			return client, nil
		case status == StatusAuthFailed:
			return nil, fmt.Errorf("%w: %s", ErrAuthFailed, lastErr)
		}

		select {
		case <-changed:
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", ErrNotConnected, ctx.Err())
		}
	}
}

// supervise keeps the session connected until ctx is cancelled.
func (s *Session) supervise(ctx context.Context) {
	backoff := initialBackoff

	for {
		if ctx.Err() != nil {
			return
		}

		s.setStatus(StatusConnecting, "")

		client, err := s.dial(ctx)
		if err != nil {
			if errors.Is(err, rcon.ErrAuthFailed) {
				// Terminal: retrying cannot help, and hammering a server with
				// a rejected password is exactly what triggers ban logic.
				s.setStatus(StatusAuthFailed, err.Error())
				return
			}

			s.setStatus(StatusDisconnected, err.Error())
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		backoff = initialBackoff
		broken := make(chan struct{})

		s.mu.Lock()
		s.client = client
		s.broken = broken
		s.brokenOnce = sync.Once{}
		s.mu.Unlock()

		s.setStatus(StatusConnected, "")

		select {
		case <-broken:
			// Execute reported a failure; loop round and reconnect.
		case <-ctx.Done():
			_ = client.Close()
			return
		}
	}
}

func (s *Session) dial(ctx context.Context) (rcon.Client, error) {
	srv, err := s.mgr.store.GetServer(ctx, s.profileID)
	if err != nil {
		return nil, err
	}
	password, err := s.mgr.store.Password(ctx, s.profileID)
	if err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, rcon.DefaultTimeout)
	defer cancel()

	return rcon.Dial(dialCtx, srv.Protocol, rcon.Target{
		Addr:     srv.Addr,
		Password: password,
	})
}
