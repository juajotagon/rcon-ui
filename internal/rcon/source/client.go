// Package source implements the Valve Source RCON dialect over TCP.
//
// It registers itself as rcon.ProtocolSource, so importing this package for
// its side effects is enough to make the dialect available:
//
//	import _ "github.com/juajotagon/rcon-ui/internal/rcon/source"
package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/juajotagon/rcon-ui/internal/rcon"
)

func init() {
	rcon.Register(rcon.ProtocolSource, dialer{})
}

type dialer struct{}

func (dialer) Dial(ctx context.Context, t rcon.Target) (rcon.Client, error) {
	return Dial(ctx, t)
}

// Client is an authenticated Source RCON connection.
//
// Safe for concurrent use: each Execute holds a mutex for the whole
// request/response exchange, because the protocol multiplexes replies on a
// single stream and interleaving two exchanges would mix their packets.
type Client struct {
	conn    net.Conn
	timeout time.Duration

	mu     sync.Mutex
	nextID int32

	// broken records that an exchange failed partway through. The remaining
	// bytes of that response are still in the socket, so the stream position
	// is unknown and every later read would be misaligned. Rather than return
	// silently wrong output, the client refuses further work and the session
	// manager reconnects.
	broken error

	closeOnce sync.Once
	closeErr  error
}

// Dial connects to a Source RCON server and authenticates. A non-nil error
// wrapping rcon.ErrAuthFailed means the password was rejected -- callers must
// not retry that, since repeatedly submitting a known-bad password can trip a
// server's ban logic.
func Dial(ctx context.Context, t rcon.Target) (*Client, error) {
	timeout := t.TimeoutOrDefault()

	var nd net.Dialer
	conn, err := nd.DialContext(ctx, "tcp", t.Addr)
	if err != nil {
		return nil, fmt.Errorf("source: dial %s: %w", t.Addr, err)
	}

	c := &Client{conn: conn, timeout: timeout}
	if err := c.authenticate(ctx, t.Password); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// authenticate performs the AUTH handshake. It runs before the Client is
// shared, so it does not take the mutex.
func (c *Client) authenticate(ctx context.Context, password string) error {
	stop := c.watchContext(ctx)
	defer stop()

	if err := c.setDeadline(ctx); err != nil {
		return err
	}

	id := c.nextRequestID()
	if err := c.write(packet{id: id, typ: typeAuth, body: password}); err != nil {
		return fmt.Errorf("source: send auth: %w", err)
	}

	for {
		p, err := decodePacket(c.conn)
		if err != nil {
			// A server that drops the connection here has almost always
			// rejected the password. Reporting a bare EOF would send the
			// session manager into a reconnect loop against a password that
			// will never work, so classify it as an auth failure.
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return fmt.Errorf("%w (server closed the connection during authentication)", rcon.ErrAuthFailed)
			}
			return fmt.Errorf("source: read auth response: %w", err)
		}

		// Several servers emit an empty RESPONSE_VALUE before the real verdict.
		// Anything that is not an auth response is noise at this stage.
		if p.typ != typeAuthResponse {
			continue
		}

		switch p.id {
		case authFailedID:
			return rcon.ErrAuthFailed
		case id:
			return nil
		default:
			return fmt.Errorf("source: auth response had id %d, want %d or %d", p.id, id, authFailedID)
		}
	}
}

// Execute runs one command and returns the complete response.
func (c *Client) Execute(ctx context.Context, cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.broken != nil {
		return "", c.broken
	}

	stop := c.watchContext(ctx)
	defer stop()

	if err := c.setDeadline(ctx); err != nil {
		return "", err
	}

	cmdID := c.nextRequestID()
	sentinelID := c.nextRequestID()

	if err := c.write(packet{id: cmdID, typ: typeExecCommand, body: cmd}); err != nil {
		return "", c.fail(fmt.Errorf("source: send command: %w", err))
	}

	// Responses longer than 4096 bytes arrive split across packets, and the
	// protocol gives no count or terminator. The standard workaround: send a
	// second, empty packet straight after the command. The server processes
	// requests in order, so once it echoes this sentinel's id, every packet
	// belonging to the real response has already arrived.
	//
	// Without this, long output (Minecraft `help`, a busy `list`) is silently
	// truncated to its first packet.
	if err := c.write(packet{id: sentinelID, typ: typeResponseValue}); err != nil {
		return "", c.fail(fmt.Errorf("source: send sentinel: %w", err))
	}

	var sb strings.Builder
	for {
		p, err := decodePacket(c.conn)
		if err != nil {
			return "", c.fail(fmt.Errorf("source: read response to %q: %w", cmd, err))
		}

		switch p.id {
		case sentinelID:
			return sb.String(), nil
		case cmdID:
			sb.WriteString(p.body)
		default:
			// A stray packet from an earlier exchange. Valve servers answer
			// the sentinel with *two* packets (the second carrying a fixed
			// 0x00 0x01 0x00 0x00 body), so one is routinely left unread.
			// Because request ids increase monotonically and are never
			// reused, such leftovers can only ever be stale, and dropping
			// them here is what keeps the stream aligned without paying for
			// a drain round-trip after every command.
			continue
		}
	}
}

// Close closes the connection. Safe to call more than once and from any
// goroutine, including while an Execute is blocked in a read.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.conn.Close()
	})
	return c.closeErr
}

func (c *Client) write(p packet) error {
	_, err := c.conn.Write(p.encode())
	return err
}

// fail records that the stream position is no longer known. Every later
// Execute returns this error rather than reading misaligned data.
func (c *Client) fail(err error) error {
	if c.broken == nil {
		c.broken = err
	}
	return err
}

// nextRequestID hands out correlation ids. It starts at 1 and skips both 0 and
// the auth-failure sentinel (-1), so a returned id is always unambiguous.
func (c *Client) nextRequestID() int32 {
	if c.nextID == math.MaxInt32 {
		c.nextID = 0
	}
	c.nextID++
	return c.nextID
}

// setDeadline bounds the exchange by the client timeout, or by the context's
// deadline when that is sooner.
func (c *Client) setDeadline(ctx context.Context) error {
	deadline := time.Now().Add(c.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := c.conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("source: set deadline: %w", err)
	}
	return nil
}

// watchContext aborts in-flight I/O when ctx is cancelled. A deadline alone
// cannot do this: cancellation can arrive long before the deadline, and a read
// already blocked in the kernel would otherwise sit there until it expires.
// The returned function must be called to release the watcher goroutine.
func (c *Client) watchContext(ctx context.Context) (stop func()) {
	if ctx.Done() == nil {
		return func() {}
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			// A deadline in the past makes any pending read or write return
			// immediately with a timeout error.
			_ = c.conn.SetDeadline(time.Now())
		case <-done:
		}
	}()
	return func() { close(done) }
}
