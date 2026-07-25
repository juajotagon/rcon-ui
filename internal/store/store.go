// Package store persists server profiles, macros and command history.
//
// SQLite via modernc.org/sqlite, which is a pure-Go translation rather than a
// cgo binding. That choice is load-bearing, not incidental: the daemon must
// build with CGO_ENABLED=0 so a single binary cross-compiles to Linux, Windows
// and macOS from one machine. Swapping in github.com/mattn/go-sqlite3 would
// silently require a cross C toolchain and break `make crosscheck`.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/juajotagon/rcon-ui/internal/rcon"
	"github.com/juajotagon/rcon-ui/internal/secret"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when an id matches no row.
var ErrNotFound = errors.New("store: not found")

// Server is one configured game server.
//
// The password is deliberately absent: it is sealed at rest and only reachable
// through Password(), so it cannot be serialised into an API response by
// accident. That is a type-level guarantee rather than a convention someone has
// to remember.
type Server struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Protocol  string    `json:"protocol"`
	Addr      string    `json:"addr"`
	Group     string    `json:"group,omitempty"`
	Game      string    `json:"game,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Macro is a saved command, either global or bound to one server.
type Macro struct {
	ID       string `json:"id"`
	ServerID string `json:"serverId,omitempty"` // empty means every server
	Name     string `json:"name"`
	Command  string `json:"command"`
	Position int    `json:"position"`
}

// HistoryEntry is one previously executed command.
type HistoryEntry struct {
	ID       int64     `json:"id"`
	ServerID string    `json:"serverId"`
	Command  string    `json:"command"`
	At       time.Time `json:"at"`
}

type Store struct {
	db     *sql.DB
	sealer secret.Sealer
}

// Open prepares the database at path, creating it and its parent directory if
// needed, and applies any outstanding migrations.
func Open(path string, sealer secret.Sealer) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("store: create data dir: %w", err)
	}

	// _busy_timeout keeps concurrent writers from failing instantly on a
	// locked database; foreign_keys makes the ON DELETE CASCADE below real,
	// since SQLite ignores foreign keys unless explicitly asked not to.
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}

	s := &Store{db: db, sealer: sealer}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// migrations are applied in order and never edited once released; a change goes
// in as a new entry. user_version tracks how many have run.
var migrations = []string{
	`CREATE TABLE servers (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		protocol   TEXT NOT NULL,
		addr       TEXT NOT NULL,
		grp        TEXT NOT NULL DEFAULT '',
		password   BLOB NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE macros (
		id        TEXT PRIMARY KEY,
		server_id TEXT REFERENCES servers(id) ON DELETE CASCADE,
		name      TEXT NOT NULL,
		command   TEXT NOT NULL,
		position  INTEGER NOT NULL DEFAULT 0
	);
	CREATE TABLE history (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
		command   TEXT NOT NULL,
		at        INTEGER NOT NULL
	);
	CREATE INDEX history_server_at ON history(server_id, at DESC);`,

	`ALTER TABLE servers ADD COLUMN game TEXT NOT NULL DEFAULT ''`,
}

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("store: read schema version: %w", err)
	}

	for i := version; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("store: begin migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: migration %d: %w", i+1, err)
		}
		// PRAGMA does not accept a bound parameter, and i+1 is a loop index
		// rather than user input, so the interpolation is safe here.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: set schema version %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: commit migration %d: %w", i+1, err)
		}
	}
	return nil
}

// newID returns a random 128-bit hex id. Random rather than sequential so ids
// leak no information about how many servers exist or the order they were
// added, and so they can be generated without consulting the database.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the system entropy source is broken;
		// there is no sensible weaker fallback for identifiers.
		panic("store: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// CreateServer seals password and stores a new profile.
func (s *Store) CreateServer(ctx context.Context, srv Server, password string) (Server, error) {
	if srv.Name == "" {
		return Server{}, errors.New("store: server name is required")
	}
	if srv.Addr == "" {
		return Server{}, errors.New("store: server address is required")
	}
	if srv.Protocol == "" {
		srv.Protocol = rcon.ProtocolSource
	}

	sealed, err := s.sealer.Seal(password)
	if err != nil {
		return Server{}, err
	}

	// Truncated to the precision actually stored (see the UnixMilli columns),
	// so the returned struct matches what a later read will produce. Without
	// this, the value handed back carries nanoseconds the database silently
	// drops, and any caller that compares or caches it sees phantom drift.
	now := time.Now().UTC().Truncate(time.Millisecond)
	srv.ID = newID()
	srv.CreatedAt = now
	srv.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO servers (id, name, protocol, addr, grp, game, password, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		srv.ID, srv.Name, srv.Protocol, srv.Addr, srv.Group, srv.Game, sealed,
		now.UnixMilli(), now.UnixMilli())
	if err != nil {
		return Server{}, fmt.Errorf("store: insert server: %w", err)
	}
	return srv, nil
}

// UpdateServer replaces a profile's mutable fields. A nil password leaves the
// stored one untouched, so an edit form need never round-trip the secret.
func (s *Store) UpdateServer(ctx context.Context, id string, srv Server, password *string) (Server, error) {
	existing, err := s.GetServer(ctx, id)
	if err != nil {
		return Server{}, err
	}

	if srv.Name != "" {
		existing.Name = srv.Name
	}
	if srv.Addr != "" {
		existing.Addr = srv.Addr
	}
	if srv.Protocol != "" {
		existing.Protocol = srv.Protocol
	}
	existing.Group = srv.Group
	existing.Game = srv.Game
	existing.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)

	if password != nil {
		sealed, err := s.sealer.Seal(*password)
		if err != nil {
			return Server{}, err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE servers SET password = ? WHERE id = ?`, sealed, id); err != nil {
			return Server{}, fmt.Errorf("store: update password: %w", err)
		}
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE servers SET name = ?, protocol = ?, addr = ?, grp = ?, game = ?, updated_at = ? WHERE id = ?`,
		existing.Name, existing.Protocol, existing.Addr, existing.Group, existing.Game,
		existing.UpdatedAt.UnixMilli(), id)
	if err != nil {
		return Server{}, fmt.Errorf("store: update server: %w", err)
	}
	return existing, nil
}

func (s *Store) DeleteServer(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete server: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetServer(ctx context.Context, id string) (Server, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, protocol, addr, grp, game, created_at, updated_at FROM servers WHERE id = ?`, id)

	srv, err := scanServer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	return srv, err
}

func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, protocol, addr, grp, game, created_at, updated_at
		 FROM servers ORDER BY grp, name`)
	if err != nil {
		return nil, fmt.Errorf("store: list servers: %w", err)
	}
	defer rows.Close()

	// Non-nil so an empty list serialises as [] rather than null.
	out := []Server{}
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

// SetGame persists a freshly discovered fingerprint into a profile's game
// field. It exists alongside UpdateServer rather than reusing it because
// UpdateServer overwrites game unconditionally (see
// TestUpdateServerGameOverwritesUnconditionally) -- fine for an edit form
// that always submits every field, wrong for the discovery pipeline, which
// only ever wants to touch this one column and must never clobber Group or
// anything else in the same request.
func (s *Store) SetGame(ctx context.Context, id, game string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE servers SET game = ?, updated_at = ? WHERE id = ?`,
		game, time.Now().UTC().Truncate(time.Millisecond).UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("store: set game: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Password unseals a stored RCON password. It is the only way out of the store,
// and callers must not log or return the result.
func (s *Store) Password(ctx context.Context, id string) (string, error) {
	var sealed []byte
	err := s.db.QueryRowContext(ctx, `SELECT password FROM servers WHERE id = ?`, id).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("store: read password: %w", err)
	}
	return s.sealer.Open(sealed)
}

// scanner covers both *sql.Row and *sql.Rows.
type scanner interface{ Scan(dest ...any) error }

func scanServer(sc scanner) (Server, error) {
	var (
		srv                  Server
		createdMS, updatedMS int64
	)
	if err := sc.Scan(&srv.ID, &srv.Name, &srv.Protocol, &srv.Addr, &srv.Group, &srv.Game, &createdMS, &updatedMS); err != nil {
		return Server{}, err
	}
	srv.CreatedAt = time.UnixMilli(createdMS).UTC()
	srv.UpdatedAt = time.UnixMilli(updatedMS).UTC()
	return srv, nil
}

// AddHistory records an executed command. Failures here must never fail the
// command itself, so callers log and continue.
func (s *Store) AddHistory(ctx context.Context, serverID, command string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO history (server_id, command, at) VALUES (?, ?, ?)`,
		serverID, command, time.Now().UTC().UnixMilli())
	if err != nil {
		return fmt.Errorf("store: add history: %w", err)
	}
	return nil
}

// ListHistory returns the most recent commands for a server, newest first.
func (s *Store) ListHistory(ctx context.Context, serverID string, limit int) ([]HistoryEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, server_id, command, at FROM history
		 WHERE server_id = ? ORDER BY at DESC, id DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list history: %w", err)
	}
	defer rows.Close()

	out := []HistoryEntry{}
	for rows.Next() {
		var (
			h    HistoryEntry
			atMS int64
		)
		if err := rows.Scan(&h.ID, &h.ServerID, &h.Command, &atMS); err != nil {
			return nil, err
		}
		h.At = time.UnixMilli(atMS).UTC()
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) CreateMacro(ctx context.Context, m Macro) (Macro, error) {
	if m.Name == "" || m.Command == "" {
		return Macro{}, errors.New("store: macro name and command are required")
	}
	m.ID = newID()

	var serverID any
	if m.ServerID != "" {
		serverID = m.ServerID
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO macros (id, server_id, name, command, position) VALUES (?, ?, ?, ?, ?)`,
		m.ID, serverID, m.Name, m.Command, m.Position)
	if err != nil {
		return Macro{}, fmt.Errorf("store: insert macro: %w", err)
	}
	return m, nil
}

// ListMacros returns macros for a server plus all global macros.
func (s *Store) ListMacros(ctx context.Context, serverID string) ([]Macro, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, COALESCE(server_id, ''), name, command, position
		 FROM macros WHERE server_id IS NULL OR server_id = ?
		 ORDER BY position, name`, serverID)
	if err != nil {
		return nil, fmt.Errorf("store: list macros: %w", err)
	}
	defer rows.Close()

	out := []Macro{}
	for rows.Next() {
		var m Macro
		if err := rows.Scan(&m.ID, &m.ServerID, &m.Name, &m.Command, &m.Position); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) DeleteMacro(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM macros WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete macro: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
