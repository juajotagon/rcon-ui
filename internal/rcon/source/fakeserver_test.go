package source

import (
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeServer speaks enough Source RCON to exercise the client, including the
// quirks real servers exhibit. It exists so CI needs no game server; the
// tradeoff is that it can only prove the client matches our reading of the
// protocol, which is why the plan also calls for a manual smoke test against
// real Minecraft and Project Zomboid servers.
type fakeServer struct {
	tb   testing.TB
	ln   net.Listener
	opts fakeOpts

	wg sync.WaitGroup
}

type fakeOpts struct {
	password string

	// handler produces the response body for a command. Defaults to echoing
	// "ok: <cmd>", which makes request/response correlation observable.
	handler func(cmd string) string

	// doubleSentinel reproduces Valve servers, which answer the end-of-response
	// sentinel with two packets rather than one. The extra packet is what would
	// desynchronise a naive client on the *following* command.
	doubleSentinel bool

	// leadingEmpty reproduces servers that emit an empty RESPONSE_VALUE before
	// the real AUTH_RESPONSE.
	leadingEmpty bool

	// closeOnBadAuth drops the connection instead of replying with id -1.
	// Several servers behave this way.
	closeOnBadAuth bool

	// hangOnCommand reads commands but never answers, so timeout and context
	// cancellation can be tested.
	hangOnCommand bool
}

func newFakeServer(tb testing.TB, opts fakeOpts) *fakeServer {
	tb.Helper()

	if opts.handler == nil {
		opts.handler = func(cmd string) string { return "ok: " + cmd }
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen: %v", err)
	}

	s := &fakeServer{tb: tb, ln: ln, opts: opts}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer conn.Close()
				s.serve(conn)
			}()
		}
	}()

	tb.Cleanup(func() {
		_ = ln.Close()
		s.wg.Wait()
	})
	return s
}

func (s *fakeServer) addr() string { return s.ln.Addr().String() }

func (s *fakeServer) serve(conn net.Conn) {
	authed := false

	for {
		p, err := decodePacket(conn)
		if err != nil {
			return // client closed, or the test ended
		}

		switch p.typ {
		case typeAuth:
			if s.opts.leadingEmpty {
				s.send(conn, packet{id: 0, typ: typeResponseValue})
			}
			if p.body != s.opts.password {
				if s.opts.closeOnBadAuth {
					return
				}
				s.send(conn, packet{id: authFailedID, typ: typeAuthResponse})
				continue
			}
			authed = true
			s.send(conn, packet{id: p.id, typ: typeAuthResponse})

		// Client->server, type 2 means EXECCOMMAND (the same value means
		// AUTH_RESPONSE in the other direction).
		case typeExecCommand:
			if !authed {
				s.send(conn, packet{id: authFailedID, typ: typeResponseValue})
				continue
			}
			if s.opts.hangOnCommand {
				continue
			}
			s.sendChunked(conn, p.id, s.opts.handler(p.body))

		// The client's end-of-response sentinel.
		case typeResponseValue:
			if s.opts.hangOnCommand {
				continue
			}
			s.send(conn, packet{id: p.id, typ: typeResponseValue})
			if s.opts.doubleSentinel {
				s.send(conn, packet{id: p.id, typ: typeResponseValue, body: "\x00\x01\x00\x00"})
			}
		}
	}
}

// sendChunked splits body across packets exactly as a real server does for
// replies over maxBodySize.
func (s *fakeServer) sendChunked(conn net.Conn, id int32, body string) {
	if body == "" {
		s.send(conn, packet{id: id, typ: typeResponseValue})
		return
	}
	for len(body) > 0 {
		n := min(len(body), maxBodySize)
		s.send(conn, packet{id: id, typ: typeResponseValue, body: body[:n]})
		body = body[n:]
	}
}

func (s *fakeServer) send(conn net.Conn, p packet) {
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(p.encode()); err != nil {
		return // the test is tearing down
	}
}

// longBody builds a response guaranteed to span several packets, with position
// markers so truncation or misordering is visible in a failure message.
func longBody(chunks int) string {
	var sb strings.Builder
	for i := range chunks * maxBodySize {
		sb.WriteByte(byte('a' + i%26))
	}
	return sb.String()
}
