// Package rcon defines the protocol-agnostic contract every RCON dialect
// implements, plus a registry so dialects can be added without touching
// callers.
//
// Only the Source dialect (internal/rcon/source) exists today. BattlEye (UDP,
// CRC32, sequence numbers) and Quake/GameSpy (plaintext UDP, password sent per
// command) are the expected next implementations; each is a new package plus a
// Register call, not a change here.
package rcon

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Protocol identifies an RCON dialect. It is stored on each server profile.
type Protocol = string

// ProtocolSource is the Valve Source RCON dialect: length-prefixed packets over
// TCP. Used by Minecraft, Project Zomboid, CS2, Rust, ARK, Palworld, Valheim
// and most other modern dedicated servers. It is the default for new profiles.
const ProtocolSource Protocol = "source"

var (
	// ErrAuthFailed means the server rejected the password. Callers should
	// surface this distinctly rather than retrying: reconnect loops must not
	// hammer a server with a password that is known to be wrong.
	ErrAuthFailed = errors.New("rcon: authentication failed")

	// ErrUnknownProtocol means no dialect is registered under that name.
	ErrUnknownProtocol = errors.New("rcon: unknown protocol")
)

// DefaultTimeout applies when a Target leaves Timeout unset. Game servers
// running a slow tick can take a surprisingly long time to answer, but a
// caller should never block forever.
const DefaultTimeout = 10 * time.Second

// Target describes one server to connect to.
type Target struct {
	// Addr is "host:port". RCON ports vary by game and are usually distinct
	// from the game's own port (Minecraft defaults to 25575).
	Addr string

	// Password is the server's RCON password. It is never logged and never
	// returned over the API.
	Password string

	// Timeout bounds each read and write. Zero means DefaultTimeout.
	Timeout time.Duration

	// TLS wraps the connection in TLS before any RCON traffic.
	//
	// RCON has no transport security of its own: the password crosses the wire
	// in the clear on the very first packet. That is tolerable on a LAN or
	// inside a cluster, and not tolerable across the internet, so exposing a
	// server usually means fronting it with a TLS-terminating proxy. This
	// speaks plain Source RCON inside the tunnel -- the dialect is unchanged,
	// only the transport differs.
	TLS bool

	// ServerName is the SNI name and the name verified against the
	// certificate. Required when the proxy routes by SNI, and when connecting
	// by IP to a host whose certificate names a domain -- without it the
	// verification fails against the IP rather than the name.
	//
	// Ignored unless TLS is set.
	ServerName string
}

// TimeoutOrDefault resolves Timeout, substituting DefaultTimeout when unset.
// Dialect packages use this so the fallback is defined in exactly one place.
func (t Target) TimeoutOrDefault() time.Duration {
	if t.Timeout <= 0 {
		return DefaultTimeout
	}
	return t.Timeout
}

// Client is one live, authenticated connection to one server.
//
// Implementations must be safe for concurrent use: the session manager may
// execute commands from several UI clients against a single connection.
type Client interface {
	// Execute runs a single command and returns the server's complete
	// response, reassembled if the dialect splits long replies.
	Execute(ctx context.Context, cmd string) (string, error)

	// Close releases the underlying connection. It is safe to call twice.
	Close() error
}

// Dialer opens authenticated connections for exactly one protocol.
type Dialer interface {
	Dial(ctx context.Context, t Target) (Client, error)
}

var (
	registryMu sync.RWMutex
	registry   = map[Protocol]Dialer{}
)

// Register makes a dialect available to Dial. It is intended to be called from
// a dialect package's init function, and panics on duplicate or empty
// registration because both indicate a programming error at wiring time.
func Register(p Protocol, d Dialer) {
	if p == "" {
		panic("rcon: Register called with empty protocol")
	}
	if d == nil {
		panic("rcon: Register called with nil Dialer for protocol " + p)
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	if _, dup := registry[p]; dup {
		panic("rcon: Register called twice for protocol " + p)
	}
	registry[p] = d
}

// Dial connects to t using the named protocol. An empty protocol is treated as
// ProtocolSource so that profiles written before other dialects existed keep
// working.
func Dial(ctx context.Context, p Protocol, t Target) (Client, error) {
	if p == "" {
		p = ProtocolSource
	}

	registryMu.RLock()
	d, ok := registry[p]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %q (available: %v)", ErrUnknownProtocol, p, Protocols())
	}
	return d.Dial(ctx, t)
}

// Protocols lists the registered dialects, sorted. The UI uses this to populate
// the protocol selector on a server profile.
func Protocols() []Protocol {
	registryMu.RLock()
	defer registryMu.RUnlock()

	out := make([]Protocol, 0, len(registry))
	for p := range registry {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
