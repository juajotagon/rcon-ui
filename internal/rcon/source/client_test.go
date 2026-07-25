package source

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/juajotagon/rcon-ui/internal/rcon"
)

const testPassword = "hunter2"

func dialTest(t *testing.T, s *fakeServer) *Client {
	t.Helper()
	c, err := Dial(t.Context(), rcon.Target{
		Addr:     s.addr(),
		Password: testPassword,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestExecute(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword})
	c := dialTest(t, s)

	got, err := c.Execute(t.Context(), "list")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := "ok: list"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The single most common wrong behaviour in RCON clients: returning only the
// first packet of a long reply.
func TestExecuteReassemblesMultiPacketResponse(t *testing.T) {
	want := longBody(3)
	s := newFakeServer(t, fakeOpts{
		password: testPassword,
		handler:  func(string) string { return want },
	})
	c := dialTest(t, s)

	got, err := c.Execute(t.Context(), "help")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got != want {
		t.Errorf("response truncated or reordered: got %d bytes, want %d", len(got), len(want))
	}
}

// Valve servers answer the sentinel with two packets. The extra one is left
// unread, so the next command must not pick it up as its own response.
func TestDoubleSentinelDoesNotDesyncNextCommand(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword, doubleSentinel: true})
	c := dialTest(t, s)

	for i := range 5 {
		cmd := fmt.Sprintf("cmd-%d", i)
		got, err := c.Execute(t.Context(), cmd)
		if err != nil {
			t.Fatalf("execute %s: %v", cmd, err)
		}
		if want := "ok: " + cmd; got != want {
			t.Fatalf("iteration %d: got %q, want %q -- stream desynchronised", i, got, want)
		}
	}
}

func TestAuthFailureReportsErrAuthFailed(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: "correct-password"})

	_, err := Dial(t.Context(), rcon.Target{Addr: s.addr(), Password: "wrong-password"})
	if !errors.Is(err, rcon.ErrAuthFailed) {
		t.Errorf("err = %v, want rcon.ErrAuthFailed", err)
	}
}

// Servers that drop the connection rather than replying -1 must still surface
// as an auth failure, or the session manager will reconnect forever against a
// password that cannot work.
func TestAuthFailureByDisconnect(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: "correct-password", closeOnBadAuth: true})

	_, err := Dial(t.Context(), rcon.Target{Addr: s.addr(), Password: "wrong-password"})
	if !errors.Is(err, rcon.ErrAuthFailed) {
		t.Errorf("err = %v, want rcon.ErrAuthFailed", err)
	}
}

func TestAuthSkipsLeadingEmptyPacket(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword, leadingEmpty: true})
	c := dialTest(t, s)

	if _, err := c.Execute(t.Context(), "list"); err != nil {
		t.Errorf("execute after leading empty packet: %v", err)
	}
}

// Each Execute holds the connection for a whole exchange, so concurrent callers
// must never receive each other's responses.
func TestConcurrentExecuteCorrelatesResponses(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword})
	c := dialTest(t, s)

	const n = 25
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := fmt.Sprintf("cmd-%d", i)
			got, err := c.Execute(t.Context(), cmd)
			if err != nil {
				errs <- err
				return
			}
			if want := "ok: " + cmd; got != want {
				errs <- fmt.Errorf("got %q, want %q", got, want)
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestExecuteRespectsContextCancellation(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword, hangOnCommand: true})
	c := dialTest(t, s)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if _, err := c.Execute(ctx, "hang"); err == nil {
		t.Fatal("expected an error when the context is cancelled")
	}
	// The client timeout is 5s; cancellation must abort well before that.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("cancellation took %v -- the context watcher did not abort the read", elapsed)
	}
}

func TestExecuteTimesOut(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword, hangOnCommand: true})

	c, err := Dial(t.Context(), rcon.Target{
		Addr:     s.addr(),
		Password: testPassword,
		Timeout:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if _, err := c.Execute(t.Context(), "hang"); err == nil {
		t.Fatal("expected a timeout error")
	}
}

// After a failed exchange the stream position is unknown, so the client must
// refuse further work rather than return misaligned data.
func TestBrokenStreamFailsSubsequentCommands(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword, hangOnCommand: true})

	c, err := Dial(t.Context(), rcon.Target{
		Addr:     s.addr(),
		Password: testPassword,
		Timeout:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if _, err := c.Execute(t.Context(), "hang"); err == nil {
		t.Fatal("expected the first command to fail")
	}
	_, err = c.Execute(t.Context(), "list")
	if err == nil {
		t.Fatal("expected the client to stay broken after a failed exchange")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword})
	c := dialTest(t, s)

	if err := c.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}

func TestDialUnreachableAddress(t *testing.T) {
	// Port 1 on loopback: nothing listens there.
	_, err := Dial(t.Context(), rcon.Target{
		Addr:     "127.0.0.1:1",
		Password: testPassword,
		Timeout:  time.Second,
	})
	if err == nil {
		t.Fatal("expected a dial error")
	}
	if errors.Is(err, rcon.ErrAuthFailed) {
		t.Error("a connection failure must not be reported as an auth failure")
	}
}

// The registry is the seam that makes adding BattlEye or Quake a new package
// rather than a refactor, so verify dialling through it works.
func TestRegistryDialByProtocol(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword})

	c, err := rcon.Dial(t.Context(), rcon.ProtocolSource, rcon.Target{
		Addr:     s.addr(),
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("dial via registry: %v", err)
	}
	defer c.Close()

	if _, err := c.Execute(t.Context(), "list"); err != nil {
		t.Errorf("execute: %v", err)
	}
}

// An empty protocol means "source", so profiles written before other dialects
// existed keep working.
func TestRegistryEmptyProtocolDefaultsToSource(t *testing.T) {
	s := newFakeServer(t, fakeOpts{password: testPassword})

	c, err := rcon.Dial(t.Context(), "", rcon.Target{Addr: s.addr(), Password: testPassword})
	if err != nil {
		t.Fatalf("dial with empty protocol: %v", err)
	}
	defer c.Close()
}

func TestRegistryUnknownProtocol(t *testing.T) {
	_, err := rcon.Dial(t.Context(), "battleye", rcon.Target{Addr: "127.0.0.1:1"})
	if !errors.Is(err, rcon.ErrUnknownProtocol) {
		t.Errorf("err = %v, want rcon.ErrUnknownProtocol", err)
	}
	// The message should help a user who typo'd a protocol name.
	if !strings.Contains(err.Error(), rcon.ProtocolSource) {
		t.Errorf("error %q should list the available protocols", err)
	}
}

func TestWithDefaultPort(t *testing.T) {
	cases := map[string]string{
		"zomboid.example.com":       "zomboid.example.com:27015",
		"zomboid.example.com:27016": "zomboid.example.com:27016",
		"192.168.1.10":              "192.168.1.10:27015",
		"192.168.1.10:25575":        "192.168.1.10:25575",
		"[::1]:25575":               "[::1]:25575",
		"[2001:db8::1]":             "[2001:db8::1]:27015",
	}
	for in, want := range cases {
		if got := withDefaultPort(in); got != want {
			t.Errorf("withDefaultPort(%q) = %q, want %q", in, got, want)
		}
	}
}
