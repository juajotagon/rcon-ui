// Package rcontest provides a fake Source RCON server for tests, in the spirit
// of net/http/httptest.
//
// The wire codec here is written independently of internal/rcon/source rather
// than importing it. That is deliberate: a fake built on the implementation
// under test cannot detect a misreading of the protocol, because it makes the
// same mistake in the same place and the two agree perfectly. Keeping a second
// implementation means the integration tests cross-check the first.
package rcontest

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// Options configures a fake server's behaviour, including quirks that real
// servers exhibit.
type Options struct {
	// Password the server accepts. Defaults to "test-password".
	Password string

	// Handler returns the response body for a command. Defaults to echoing.
	Handler func(cmd string) string

	// DoubleSentinel reproduces Valve servers, which answer the
	// end-of-response sentinel with two packets rather than one.
	DoubleSentinel bool

	// Hang accepts commands but never answers them, for timeout tests.
	Hang bool
}

// Server is a fake RCON server listening on loopback.
type Server struct {
	ln   net.Listener
	opts Options
	wg   sync.WaitGroup

	mu       sync.Mutex
	commands []string
}

// Start launches a server and registers cleanup with tb.
func Start(tb testing.TB, opts Options) *Server {
	tb.Helper()

	if opts.Password == "" {
		opts.Password = "test-password"
	}
	if opts.Handler == nil {
		opts.Handler = func(cmd string) string { return "ok: " + cmd }
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("rcontest: listen: %v", err)
	}

	s := &Server{ln: ln, opts: opts}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer conn.Close()
				s.serve(conn)
			}()
		}
	}()

	tb.Cleanup(s.Close)
	return s
}

// Addr is the host:port to connect to.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Password is the password the server accepts.
func (s *Server) Password() string { return s.opts.Password }

// Commands returns every command the server has received, in order. Useful for
// asserting what actually reached the wire.
func (s *Server) Commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}

func (s *Server) Close() {
	_ = s.ln.Close()
	s.wg.Wait()
}

func (s *Server) serve(conn net.Conn) {
	authed := false

	for {
		id, typ, body, err := readPacket(conn)
		if err != nil {
			return
		}

		switch typ {
		case 3: // SERVERDATA_AUTH
			if body != s.opts.Password {
				writePacket(conn, -1, 2, "")
				continue
			}
			authed = true
			writePacket(conn, id, 2, "")

		case 2: // SERVERDATA_EXECCOMMAND
			if !authed {
				writePacket(conn, -1, 0, "")
				continue
			}
			s.mu.Lock()
			s.commands = append(s.commands, body)
			s.mu.Unlock()

			if s.opts.Hang {
				continue
			}
			for _, chunk := range split(s.opts.Handler(body), 4096) {
				writePacket(conn, id, 0, chunk)
			}

		case 0: // the client's end-of-response sentinel
			if s.opts.Hang {
				continue
			}
			writePacket(conn, id, 0, "")
			if s.opts.DoubleSentinel {
				writePacket(conn, id, 0, "\x00\x01\x00\x00")
			}
		}
	}
}

// LongResponse builds a body spanning n packets, for exercising reassembly.
func LongResponse(n int) string {
	var sb strings.Builder
	for i := range n * 4096 {
		fmt.Fprintf(&sb, "%c", 'a'+i%26)
	}
	return sb.String()
}

func split(s string, n int) []string {
	if s == "" {
		return []string{""}
	}
	var out []string
	for len(s) > 0 {
		i := min(n, len(s))
		out = append(out, s[:i])
		s = s[i:]
	}
	return out
}

func readPacket(conn net.Conn) (id, typ int32, body string, err error) {
	var lenBuf [4]byte
	if _, err = io.ReadFull(conn, lenBuf[:]); err != nil {
		return
	}

	n := binary.LittleEndian.Uint32(lenBuf[:])
	if n < 10 || n > 1<<20 {
		err = fmt.Errorf("rcontest: bad packet length %d", n)
		return
	}

	buf := make([]byte, n)
	if _, err = io.ReadFull(conn, buf); err != nil {
		return
	}
	id = int32(binary.LittleEndian.Uint32(buf[0:4]))
	typ = int32(binary.LittleEndian.Uint32(buf[4:8]))
	body = string(bytes.TrimRight(buf[8:], "\x00"))
	return
}

func writePacket(conn net.Conn, id, typ int32, body string) {
	n := 4 + 4 + len(body) + 2

	buf := make([]byte, 0, 4+n)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(n))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(id))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(typ))
	buf = append(buf, body...)
	buf = append(buf, 0, 0)

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, _ = conn.Write(buf)
}
