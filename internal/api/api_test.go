package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/juajotagon/rcon-ui/internal/api"
	"github.com/juajotagon/rcon-ui/internal/discovery"
	"github.com/juajotagon/rcon-ui/internal/event"
	"github.com/juajotagon/rcon-ui/internal/rcontest"
	"github.com/juajotagon/rcon-ui/internal/secret"
	"github.com/juajotagon/rcon-ui/internal/session"
	"github.com/juajotagon/rcon-ui/internal/store"

	_ "github.com/juajotagon/rcon-ui/internal/rcon/source"
)

type harness struct {
	http  *httptest.Server
	rcon  *rcontest.Server
	store *store.Store
	hub   *event.Hub
}

func newHarness(tb testing.TB, token string, opts rcontest.Options) *harness {
	tb.Helper()

	fake := rcontest.Start(tb, opts)

	sealer, err := secret.NewFromKey([]byte("test-key-for-unit-tests"))
	if err != nil {
		tb.Fatalf("sealer: %v", err)
	}

	st, err := store.Open(filepath.Join(tb.TempDir(), "test.db"), sealer)
	if err != nil {
		tb.Fatalf("store: %v", err)
	}
	tb.Cleanup(func() { st.Close() })

	hub := event.NewHub()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := session.NewManager(st, hub, log)
	tb.Cleanup(mgr.Close)

	srv := httptest.NewServer(api.New(st, hub, mgr, log, token, nil).Handler())
	tb.Cleanup(srv.Close)

	return &harness{http: srv, rcon: fake, store: st, hub: hub}
}

func (h *harness) do(tb testing.TB, method, path string, body any) (*http.Response, []byte) {
	tb.Helper()

	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			tb.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, h.http.URL+path, rdr)
	if err != nil {
		tb.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.http.Client().Do(req)
	if err != nil {
		tb.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

// createServer registers the fake RCON server as a profile and returns its id.
func (h *harness) createServer(tb testing.TB) string {
	tb.Helper()

	resp, body := h.do(tb, "POST", "/api/servers", map[string]any{
		"name":     "test-server",
		"addr":     h.rcon.Addr(),
		"password": h.rcon.Password(),
	})
	if resp.StatusCode != http.StatusCreated {
		tb.Fatalf("create server: %d %s", resp.StatusCode, body)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		tb.Fatalf("unmarshal: %v", err)
	}
	return created.ID
}

func TestServerCRUD(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{})
	id := h.createServer(t)

	resp, body := h.do(t, "GET", "/api/servers", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: %d %s", resp.StatusCode, body)
	}

	var list []map[string]any
	json.Unmarshal(body, &list)
	if len(list) != 1 {
		t.Fatalf("got %d servers, want 1", len(list))
	}
	if list[0]["status"] != string(session.StatusDisconnected) {
		t.Errorf("status = %v, want disconnected", list[0]["status"])
	}

	resp, body = h.do(t, "PATCH", "/api/servers/"+id, map[string]any{"name": "renamed"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch: %d %s", resp.StatusCode, body)
	}

	if resp, _ = h.do(t, "DELETE", "/api/servers/"+id, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete: %d", resp.StatusCode)
	}
	if resp, _ = h.do(t, "GET", "/api/servers/"+id, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete: %d, want 404", resp.StatusCode)
	}
}

// The game field round-trips through create, update and clear, and it must
// come back out of every response that carries a profile -- it flows through
// serverResponse's embedded store.Server, so this also pins that the API
// layer doesn't need its own copy of the field.
func TestServerGameField(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{})

	resp, body := h.do(t, "POST", "/api/servers", map[string]any{
		"name": "pz", "addr": h.rcon.Addr(), "password": h.rcon.Password(), "game": "zomboid",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var created struct {
		ID   string `json:"id"`
		Game string `json:"game"`
	}
	json.Unmarshal(body, &created)
	if created.Game != "zomboid" {
		t.Errorf("game = %q, want zomboid", created.Game)
	}

	resp, body = h.do(t, "PATCH", "/api/servers/"+created.ID, map[string]any{"name": "pz", "game": "minecraft"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: %d %s", resp.StatusCode, body)
	}
	var updated struct {
		Game string `json:"game"`
	}
	json.Unmarshal(body, &updated)
	if updated.Game != "minecraft" {
		t.Errorf("game = %q, want minecraft", updated.Game)
	}

	// Omitting game from the request body must clear it, matching Group's
	// overwrite semantics -- there is no separate "leave alone" signal.
	resp, body = h.do(t, "PATCH", "/api/servers/"+created.ID, map[string]any{"name": "pz"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear: %d %s", resp.StatusCode, body)
	}
	var cleared struct {
		Game string `json:"game"`
	}
	json.Unmarshal(body, &cleared)
	if cleared.Game != "" {
		t.Errorf("game = %q, want cleared", cleared.Game)
	}
}

// TLS and ServerName round-trip through create and come back out of the
// response, matching how Game already flows through serverResponse's embedded
// store.Server.
func TestServerTLSFields(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{})

	resp, body := h.do(t, "POST", "/api/servers", map[string]any{
		"name": "pz", "addr": h.rcon.Addr(), "password": h.rcon.Password(),
		"tls": true, "serverName": "zomboid.juanjo.site",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var created struct {
		ID         string `json:"id"`
		TLS        bool   `json:"tls"`
		ServerName string `json:"serverName"`
	}
	json.Unmarshal(body, &created)
	if !created.TLS || created.ServerName != "zomboid.juanjo.site" {
		t.Errorf("got tls=%v serverName=%q, want tls=true serverName=zomboid.juanjo.site", created.TLS, created.ServerName)
	}

	resp, body = h.do(t, "GET", "/api/servers/"+created.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: %d %s", resp.StatusCode, body)
	}
	var got struct {
		TLS        bool   `json:"tls"`
		ServerName string `json:"serverName"`
	}
	json.Unmarshal(body, &got)
	if !got.TLS || got.ServerName != "zomboid.juanjo.site" {
		t.Errorf("got tls=%v serverName=%q, want tls=true serverName=zomboid.juanjo.site", got.TLS, got.ServerName)
	}
}

// The password must never come back out of the API, in any response.
func TestPasswordNeverReturned(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{})
	h.createServer(t)

	for _, path := range []string{"/api/servers", "/api/servers/"} {
		_, body := h.do(t, "GET", path, nil)
		if bytes.Contains(bytes.ToLower(body), []byte(h.rcon.Password())) {
			t.Errorf("%s leaked the password: %s", path, body)
		}
	}
}

func TestExecuteCommand(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{})
	id := h.createServer(t)

	resp, body := h.do(t, "POST", "/api/servers/"+id+"/execute", map[string]any{"command": "list"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("execute: %d %s", resp.StatusCode, body)
	}

	var out struct {
		Response string `json:"response"`
	}
	json.Unmarshal(body, &out)
	if want := "ok: list"; out.Response != want {
		t.Errorf("response = %q, want %q", out.Response, want)
	}

	// Connecting now also fires an automatic discovery run in the background
	// (showoptions, help, ...), so the wire no longer carries only what this
	// test asked for -- just that "list" is among what it sent, not that it
	// was alone.
	found := false
	for _, cmd := range h.rcon.Commands() {
		if cmd == "list" {
			found = true
		}
	}
	if !found {
		t.Errorf("server did not receive %q, got %v", "list", h.rcon.Commands())
	}
}

// This is the check that proves the v2 log seam survived Phase 2.
//
// A command and its reply must arrive as TWO separately-tagged events with
// monotonic Seq -- not one {command, response} pair. The paired shape is the
// one that cannot represent a log line, which has no command half, and adopting
// it would force the hub and the console to be rewritten when log sources land.
func TestEventStreamEmitsSeparateCommandAndResponseEvents(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{})
	id := h.createServer(t)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", h.http.URL+"/api/events?profileId="+id, nil)
	resp, err := h.http.Client().Do(req)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	events := make(chan event.Event, 32)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var e event.Event
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e) == nil {
				events <- e
			}
		}
	}()

	h.do(t, "POST", "/api/servers/"+id+"/execute", map[string]any{"command": "list"})

	var command, response *event.Event
	deadline := time.After(5 * time.Second)
	for command == nil || response == nil {
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatal("event stream closed early")
			}
			switch e.Stream {
			case event.StreamCommand:
				command = &e
			case event.StreamResponse:
				response = &e
			}
		case <-deadline:
			t.Fatalf("timed out; got command=%v response=%v", command, response)
		}
	}

	if command.Line != "list" {
		t.Errorf("command line = %q, want %q", command.Line, "list")
	}
	if response.Line != "ok: list" {
		t.Errorf("response line = %q, want %q", response.Line, "ok: list")
	}
	if command.Source != event.SourceRCON || response.Source != event.SourceRCON {
		t.Errorf("both events must be tagged source=rcon, got %q and %q", command.Source, response.Source)
	}
	if command.Seq >= response.Seq {
		t.Errorf("Seq must increase: command=%d response=%d", command.Seq, response.Seq)
	}
	if command.ProfileID != id || response.ProfileID != id {
		t.Errorf("events must carry the profile id, got %q and %q", command.ProfileID, response.ProfileID)
	}
}

// Resumption is what makes an SSE drop invisible to the user, and it is the
// reason Seq is globally monotonic rather than per-profile.
func TestEventStreamResumesFromLastEventID(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{})
	id := h.createServer(t)

	first := h.hub.Publish(event.Event{ProfileID: id, Source: event.SourceRCON, Stream: event.StreamResponse, Line: "before"})
	second := h.hub.Publish(event.Event{ProfileID: id, Source: event.SourceRCON, Stream: event.StreamResponse, Line: "after"})

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/events?profileId=%s&lastEventId=%d", h.http.URL, id, first.Seq)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := h.http.Client().Do(req)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer resp.Body.Close()

	// Read only the replayed portion, then stop.
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	got := string(buf[:n])

	if strings.Contains(got, `"before"`) {
		t.Error("replayed an event the client had already seen")
	}
	if !strings.Contains(got, `"after"`) {
		t.Errorf("did not replay the missed event; got: %s", got)
	}
	if !strings.Contains(got, fmt.Sprintf("id: %d", second.Seq)) {
		t.Errorf("missing SSE id field for resumption; got: %s", got)
	}
}

func TestAuthTokenRequired(t *testing.T) {
	const token = "s3cret-token"
	h := newHarness(t, token, rcontest.Options{})

	if resp, _ := h.do(t, "GET", "/api/servers", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token: %d, want 401", resp.StatusCode)
	}

	req, _ := http.NewRequest("GET", h.http.URL+"/api/servers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := h.http.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("with header: %d, want 200", resp.StatusCode)
	}

	// EventSource cannot set headers, so the query form has to work or the
	// event stream is unauthenticatable from a browser.
	resp2, err := h.http.Client().Get(h.http.URL + "/api/servers?access_token=" + token)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("with query token: %d, want 200", resp2.StatusCode)
	}
}

// Regression: gating static assets on the bearer token made the desktop build
// open a blank window. The document is reached at /?access_token=…, but a
// browser attaches nothing to the <script> and <link> requests it then makes,
// so they 401 and the application never boots. Only /api/ may require the
// token.
func TestStaticAssetsAreServedWithoutToken(t *testing.T) {
	const token = "s3cret-token"

	static := fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<!doctype html><div id=root>")},
		"assets/app.js":  &fstest.MapFile{Data: []byte("console.log(1)")},
		"assets/app.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	sealer, _ := secret.NewFromKey([]byte("k"))
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"), sealer)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	hub := event.NewHub()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := session.NewManager(st, hub, log)
	t.Cleanup(mgr.Close)

	srv := httptest.NewServer(api.New(st, hub, mgr, log, token, static).Handler())
	t.Cleanup(srv.Close)

	// Exactly what a webview requests after navigating: no credentials at all.
	for _, path := range []string{"/", "/index.html", "/assets/app.js", "/assets/app.css"} {
		resp, err := srv.Client().Get(srv.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s = %d without a token, want 200; the UI cannot load otherwise", path, resp.StatusCode)
		}
	}

	// The API stays protected.
	resp, err := srv.Client().Get(srv.URL + "/api/servers")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("/api/servers = %d without a token, want 401", resp.StatusCode)
	}
}

func TestExecuteWithBadPasswordReturns401(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{Password: "correct"})

	resp, body := h.do(t, "POST", "/api/servers", map[string]any{
		"name": "bad-pass", "addr": h.rcon.Addr(), "password": "wrong",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.Unmarshal(body, &created)

	resp, body = h.do(t, "POST", "/api/servers/"+created.ID+"/execute", map[string]any{"command": "list"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("execute with bad password: %d, want 401; body: %s", resp.StatusCode, body)
	}
}

// Long output must survive the whole path: RCON reassembly, the event hub,
// JSON encoding and SSE framing.
func TestMultiPacketResponseSurvivesFullStack(t *testing.T) {
	want := rcontest.LongResponse(3)
	h := newHarness(t, "", rcontest.Options{
		DoubleSentinel: true,
		Handler:        func(string) string { return want },
	})
	id := h.createServer(t)

	resp, body := h.do(t, "POST", "/api/servers/"+id+"/execute", map[string]any{"command": "help"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("execute: %d %s", resp.StatusCode, body)
	}

	var out struct {
		Response string `json:"response"`
	}
	json.Unmarshal(body, &out)
	if len(out.Response) != len(want) {
		t.Errorf("response = %d bytes, want %d", len(out.Response), len(want))
	}
}

func TestHistoryRecorded(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{})
	id := h.createServer(t)

	for _, cmd := range []string{"list", "time set day"} {
		h.do(t, "POST", "/api/servers/"+id+"/execute", map[string]any{"command": cmd})
	}

	_, body := h.do(t, "GET", "/api/servers/"+id+"/history", nil)
	var entries []store.HistoryEntry
	json.Unmarshal(body, &entries)

	if len(entries) != 2 {
		t.Fatalf("got %d history entries, want 2: %s", len(entries), body)
	}
	// Newest first.
	if entries[0].Command != "time set day" {
		t.Errorf("history[0] = %q, want %q", entries[0].Command, "time set day")
	}
}

func TestMacros(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{})
	id := h.createServer(t)

	resp, body := h.do(t, "POST", "/api/macros", map[string]any{
		"serverId": id, "name": "Save", "command": "save-all",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create macro: %d %s", resp.StatusCode, body)
	}

	_, body = h.do(t, "GET", "/api/macros?serverId="+id, nil)
	var macros []store.Macro
	json.Unmarshal(body, &macros)
	if len(macros) != 1 || macros[0].Command != "save-all" {
		t.Fatalf("got %+v, want one save-all macro", macros)
	}

	if resp, _ = h.do(t, "DELETE", "/api/macros/"+macros[0].ID, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete macro: %d", resp.StatusCode)
	}
}

// pzHandler answers showoptions/help exactly as a real Project Zomboid server
// does, reusing the transcripts internal/discovery already pins its own
// parser tests against rather than duplicating them here.
func pzHandler(t testing.TB) func(cmd string) string {
	t.Helper()
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join("..", "discovery", "testdata", name))
		if err != nil {
			t.Fatalf("read testdata %s: %v", name, err)
		}
		return string(data)
	}
	showoptions := read("pz_showoptions.txt")
	help := read("pz_help.txt")

	return func(cmd string) string {
		switch cmd {
		case "showoptions":
			return showoptions
		case "help":
			return help
		default:
			return "unknown command"
		}
	}
}

// TestDiscoveryEndToEnd exercises the whole discovery contract through the
// HTTP API: nothing before any run, the automatic post-connect run
// populating the cache asynchronously, a manual re-run, and 409 once the
// session drops.
func TestDiscoveryEndToEnd(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{Handler: pzHandler(t)})
	id := h.createServer(t)

	// No discovery has run yet: 404, not an empty report.
	resp, body := h.do(t, "GET", "/api/servers/"+id+"/discovery", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("discovery before connect: %d %s", resp.StatusCode, body)
	}
	var notYet map[string]string
	json.Unmarshal(body, &notYet)
	if notYet["error"] != "no discovery yet" {
		t.Errorf("error = %q, want %q", notYet["error"], "no discovery yet")
	}

	if resp, body := h.do(t, "POST", "/api/servers/"+id+"/connect", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("connect: %d %s", resp.StatusCode, body)
	}

	// Discovery runs in a goroutine after connect+auth, so poll briefly
	// rather than assuming it has finished by the time connect returns.
	var report discovery.Report
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, body := h.do(t, "GET", "/api/servers/"+id+"/discovery", nil)
		if resp.StatusCode == http.StatusOK {
			if err := json.Unmarshal(body, &report); err != nil {
				t.Fatalf("unmarshal report: %v", err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("discovery never became available: last status %d %s", resp.StatusCode, body)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if report.Fingerprint != discovery.FingerprintZomboid {
		t.Errorf("fingerprint = %q, want %q", report.Fingerprint, discovery.FingerprintZomboid)
	}
	if len(report.Options) == 0 || len(report.Commands) == 0 {
		t.Errorf("expected non-empty options and commands, got %d and %d", len(report.Options), len(report.Commands))
	}
	for _, o := range report.Options {
		if o.Key == "Password" || o.Key == "AdminPassword" {
			t.Errorf("credential-shaped option %q leaked through the API", o.Key)
		}
	}

	// The profile's persisted fingerprint follows the discovered one.
	_, body = h.do(t, "GET", "/api/servers/"+id, nil)
	var withGame struct {
		Game string `json:"game"`
	}
	json.Unmarshal(body, &withGame)
	if withGame.Game != "zomboid" {
		t.Errorf("persisted game = %q, want zomboid", withGame.Game)
	}

	// A manual re-run returns fresh data synchronously -- no polling needed.
	resp, body = h.do(t, "POST", "/api/servers/"+id+"/discover", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manual discover: %d %s", resp.StatusCode, body)
	}
	var rerun discovery.Report
	json.Unmarshal(body, &rerun)
	if rerun.Fingerprint != discovery.FingerprintZomboid {
		t.Errorf("rerun fingerprint = %q, want %q", rerun.Fingerprint, discovery.FingerprintZomboid)
	}

	// Disconnected, discovery has nothing live to probe: 409.
	h.do(t, "POST", "/api/servers/"+id+"/disconnect", nil)
	resp, body = h.do(t, "POST", "/api/servers/"+id+"/discover", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("discover while disconnected: %d %s, want 409", resp.StatusCode, body)
	}
	var notConnected map[string]string
	json.Unmarshal(body, &notConnected)
	if notConnected["error"] != "not connected" {
		t.Errorf("error = %q, want %q", notConnected["error"], "not connected")
	}

	// The cache from before the disconnect is still there -- GET is a pure
	// cache read and must not be affected by the session dropping.
	resp, body = h.do(t, "GET", "/api/servers/"+id+"/discovery", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("discovery after disconnect: %d %s, want cached 200", resp.StatusCode, body)
	}
}

// TestDiscoveryEventFiresOnAutomaticAndManualRuns pins a resolved ambiguity in
// the discovery contract: the spec's prose calls out only the automatic
// post-connect run publishing the summary event, but a manual re-scan is
// otherwise invisible to any other browser tab watching the same profile's
// event stream -- and the frontend's own App.tsx is written expecting a
// discovery event from "this client or another client of the same daemon".
// Both paths publish.
func TestDiscoveryEventFiresOnAutomaticAndManualRuns(t *testing.T) {
	h := newHarness(t, "", rcontest.Options{Handler: pzHandler(t)})
	id := h.createServer(t)

	events, unsubscribe := h.hub.Subscribe(id)
	defer unsubscribe()

	if resp, body := h.do(t, "POST", "/api/servers/"+id+"/connect", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("connect: %d %s", resp.StatusCode, body)
	}

	waitForDiscoveryEvent := func() event.Event {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			select {
			case e := <-events:
				if e.Stream == event.StreamDiscovery {
					return e
				}
			case <-deadline:
				t.Fatal("timed out waiting for a discovery event")
				return event.Event{}
			}
		}
	}

	first := waitForDiscoveryEvent()
	if !strings.Contains(first.Line, "Project Zomboid") {
		t.Errorf("automatic discovery event line = %q, want it to mention Project Zomboid", first.Line)
	}
	if first.Source != event.SourceSystem {
		t.Errorf("source = %q, want %q", first.Source, event.SourceSystem)
	}

	if resp, body := h.do(t, "POST", "/api/servers/"+id+"/discover", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("manual discover: %d %s", resp.StatusCode, body)
	}
	second := waitForDiscoveryEvent()
	if second.Seq <= first.Seq {
		t.Errorf("manual discover's event Seq = %d, want greater than the automatic run's %d", second.Seq, first.Seq)
	}
}
