package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/juajotagon/rcon-ui/internal/rcon"
	"github.com/juajotagon/rcon-ui/internal/secret"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	sealer, err := secret.NewFromKey([]byte("test-key"))
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}

	st, err := Open(filepath.Join(t.TempDir(), "test.db"), sealer)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestCreateAndGetServer(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, err := st.CreateServer(ctx, Server{Name: "mc", Addr: "host:25575"}, "hunter2")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" {
		t.Error("no id assigned")
	}
	// An unspecified protocol must default, so profiles written before other
	// dialects existed keep working.
	if created.Protocol != rcon.ProtocolSource {
		t.Errorf("protocol = %q, want %q", created.Protocol, rcon.ProtocolSource)
	}

	got, err := st.GetServer(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "mc" || got.Addr != "host:25575" {
		t.Errorf("got %+v", got)
	}
}

func TestPasswordRoundTripsSealed(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, _ := st.CreateServer(ctx, Server{Name: "mc", Addr: "h:1"}, "hunter2")

	got, err := st.Password(ctx, created.ID)
	if err != nil {
		t.Fatalf("password: %v", err)
	}
	if got != "hunter2" {
		t.Errorf("password = %q, want hunter2", got)
	}
}

// The Server struct has no password field at all, so it cannot be serialised
// into an API response by accident. This pins that as a guarantee.
func TestServerStructHasNoPasswordField(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, _ := st.CreateServer(ctx, Server{Name: "mc", Addr: "h:1"}, "hunter2")
	got, _ := st.GetServer(ctx, created.ID)

	if got != created {
		t.Errorf("round trip changed the struct:\n got %+v\nwant %+v", got, created)
	}
}

func TestCreateServerValidates(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	if _, err := st.CreateServer(ctx, Server{Addr: "h:1"}, "p"); err == nil {
		t.Error("expected an error for a missing name")
	}
	if _, err := st.CreateServer(ctx, Server{Name: "n"}, "p"); err == nil {
		t.Error("expected an error for a missing address")
	}
}

func TestUpdateServerKeepsPasswordWhenOmitted(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, _ := st.CreateServer(ctx, Server{Name: "mc", Addr: "h:1"}, "original")

	if _, err := st.UpdateServer(ctx, created.ID, Server{Name: "renamed"}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := st.Password(ctx, created.ID)
	if got != "original" {
		t.Errorf("password = %q; omitting it must leave the stored one alone", got)
	}

	srv, _ := st.GetServer(ctx, created.ID)
	if srv.Name != "renamed" {
		t.Errorf("name = %q, want renamed", srv.Name)
	}
	// Fields not supplied must survive the update.
	if srv.Addr != "h:1" {
		t.Errorf("addr = %q, want h:1", srv.Addr)
	}
}

func TestUpdateServerChangesPasswordWhenGiven(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, _ := st.CreateServer(ctx, Server{Name: "mc", Addr: "h:1"}, "original")

	updated := "replacement"
	if _, err := st.UpdateServer(ctx, created.ID, Server{}, &updated); err != nil {
		t.Fatalf("update: %v", err)
	}

	if got, _ := st.Password(ctx, created.ID); got != "replacement" {
		t.Errorf("password = %q, want replacement", got)
	}
}

// Game follows the same overwrite semantics as Group: an update replaces it
// unconditionally, including with the empty string, so a profile can be
// switched back to console-only.
func TestUpdateServerGameOverwritesUnconditionally(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, err := st.CreateServer(ctx, Server{Name: "pz", Addr: "h:1", Game: "zomboid"}, "p")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Game != "zomboid" {
		t.Errorf("game = %q, want zomboid", created.Game)
	}

	updated, err := st.UpdateServer(ctx, created.ID, Server{Name: "pz", Game: "minecraft"}, nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Game != "minecraft" {
		t.Errorf("game = %q, want minecraft", updated.Game)
	}

	cleared, err := st.UpdateServer(ctx, created.ID, Server{Name: "pz", Game: ""}, nil)
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared.Game != "" {
		t.Errorf("game = %q, want cleared", cleared.Game)
	}
}

// SetGame is the discovery pipeline's write path: unlike UpdateServer, it must
// touch only the game column. A profile with a Group set is the regression
// case -- UpdateServer's unconditional overwrite would silently clear it.
func TestSetGame(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, err := st.CreateServer(ctx, Server{Name: "pz", Addr: "h:1", Group: "survival"}, "p")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := st.SetGame(ctx, created.ID, "zomboid"); err != nil {
		t.Fatalf("set game: %v", err)
	}

	got, err := st.GetServer(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Game != "zomboid" {
		t.Errorf("game = %q, want zomboid", got.Game)
	}
	if got.Group != "survival" {
		t.Errorf("group = %q, want unchanged %q", got.Group, "survival")
	}

	if err := st.SetGame(ctx, "does-not-exist", "zomboid"); !errors.Is(err, ErrNotFound) {
		t.Errorf("set game on missing profile: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteServer(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, _ := st.CreateServer(ctx, Server{Name: "mc", Addr: "h:1"}, "p")

	if err := st.DeleteServer(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := st.GetServer(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if err := st.DeleteServer(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: err = %v, want ErrNotFound", err)
	}
}

func TestListServersEmptyIsNotNil(t *testing.T) {
	st := newTestStore(t)

	got, err := st.ListServers(t.Context())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// Nil would serialise as JSON null, which the frontend would have to
	// special-case on every render.
	if got == nil {
		t.Error("empty list is nil; it must serialise as []")
	}
}

func TestHistoryNewestFirst(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, _ := st.CreateServer(ctx, Server{Name: "mc", Addr: "h:1"}, "p")
	for _, cmd := range []string{"first", "second", "third"} {
		if err := st.AddHistory(ctx, created.ID, cmd); err != nil {
			t.Fatalf("add history: %v", err)
		}
	}

	entries, err := st.ListHistory(ctx, created.ID, 0)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if entries[0].Command != "third" {
		t.Errorf("newest entry = %q, want third", entries[0].Command)
	}
}

// Deleting a server must not leave its history and macros behind, which is what
// the foreign_keys pragma buys.
func TestDeleteServerCascades(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	created, _ := st.CreateServer(ctx, Server{Name: "mc", Addr: "h:1"}, "p")
	st.AddHistory(ctx, created.ID, "list")
	st.CreateMacro(ctx, Macro{ServerID: created.ID, Name: "Save", Command: "save-all"})

	if err := st.DeleteServer(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if entries, _ := st.ListHistory(ctx, created.ID, 0); len(entries) != 0 {
		t.Errorf("history survived the server: %d entries", len(entries))
	}
	if macros, _ := st.ListMacros(ctx, created.ID); len(macros) != 0 {
		t.Errorf("macros survived the server: %d", len(macros))
	}
}

func TestMacrosGlobalAndPerServer(t *testing.T) {
	st := newTestStore(t)
	ctx := t.Context()

	a, _ := st.CreateServer(ctx, Server{Name: "a", Addr: "h:1"}, "p")
	b, _ := st.CreateServer(ctx, Server{Name: "b", Addr: "h:2"}, "p")

	if _, err := st.CreateMacro(ctx, Macro{Name: "Global", Command: "help"}); err != nil {
		t.Fatalf("create global macro: %v", err)
	}
	if _, err := st.CreateMacro(ctx, Macro{ServerID: a.ID, Name: "OnlyA", Command: "save-all"}); err != nil {
		t.Fatalf("create scoped macro: %v", err)
	}

	// Server a sees its own macro plus the global one.
	macros, _ := st.ListMacros(ctx, a.ID)
	if len(macros) != 2 {
		t.Errorf("server a: got %d macros, want 2", len(macros))
	}

	// Server b sees only the global one.
	macros, _ = st.ListMacros(ctx, b.ID)
	if len(macros) != 1 || macros[0].Name != "Global" {
		t.Errorf("server b: got %+v, want only the global macro", macros)
	}
}

// Reopening must not re-run migrations or lose data.
func TestMigrationsAreIdempotent(t *testing.T) {
	sealer, _ := secret.NewFromKey([]byte("k"))
	path := filepath.Join(t.TempDir(), "test.db")

	first, err := Open(path, sealer)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	created, _ := first.CreateServer(t.Context(), Server{Name: "mc", Addr: "h:1"}, "p")
	first.Close()

	second, err := Open(path, sealer)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()

	if _, err := second.GetServer(t.Context(), created.ID); err != nil {
		t.Errorf("data lost across reopen: %v", err)
	}
}

// A database written before the game column existed must migrate cleanly:
// the ALTER TABLE runs, existing rows default to an empty game, and both
// reading and writing the field work afterwards.
func TestMigrationAddsGameColumnToExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := raw.Exec(migrations[0]); err != nil {
		t.Fatalf("apply v1 schema: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO servers (id, name, protocol, addr, grp, password, created_at, updated_at)
		 VALUES ('old-id', 'legacy', 'source', 'h:1', '', x'00', 0, 0)`); err != nil {
		t.Fatalf("seed pre-migration row: %v", err)
	}
	if _, err := raw.Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	sealer, _ := secret.NewFromKey([]byte("k"))
	st, err := Open(path, sealer)
	if err != nil {
		t.Fatalf("open with migration: %v", err)
	}
	defer st.Close()

	srv, err := st.GetServer(t.Context(), "old-id")
	if err != nil {
		t.Fatalf("get pre-existing row: %v", err)
	}
	if srv.Game != "" {
		t.Errorf("game = %q, want empty default for a row predating the column", srv.Game)
	}

	created, err := st.CreateServer(t.Context(), Server{Name: "new", Addr: "h:2", Game: "zomboid"}, "p")
	if err != nil {
		t.Fatalf("create after migration: %v", err)
	}
	if created.Game != "zomboid" {
		t.Errorf("game = %q, want zomboid", created.Game)
	}
}
