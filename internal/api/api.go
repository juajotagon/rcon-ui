// Package api serves the REST endpoints and the event stream.
//
// Streaming uses Server-Sent Events rather than WebSocket. The plan specified
// WebSocket, and this is a deliberate, documented departure: the streaming flow
// here is strictly server-to-client (status changes and console events), while
// commands travel client-to-server over ordinary POSTs. SSE covers exactly that
// shape using only net/http, whereas WebSocket would add a dependency to a
// project whose whole pitch is a single zero-dependency binary. The browser's
// EventSource also reconnects and resumes from Last-Event-ID for free, which
// would otherwise be hand-written.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/juajotagon/rcon-ui/internal/event"
	"github.com/juajotagon/rcon-ui/internal/rcon"
	"github.com/juajotagon/rcon-ui/internal/session"
	"github.com/juajotagon/rcon-ui/internal/store"
)

type Server struct {
	store *store.Store
	hub   *event.Hub
	mgr   *session.Manager
	log   *slog.Logger
	token string

	// static holds the built frontend. Nil until Phase 3 wires the embed, in
	// which case only the API is served.
	static fs.FS
}

func New(st *store.Store, hub *event.Hub, mgr *session.Manager, log *slog.Logger, token string, static fs.FS) *Server {
	return &Server{store: st, hub: hub, mgr: mgr, log: log, token: token, static: static}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/protocols", s.handleProtocols)

	mux.HandleFunc("GET /api/servers", s.handleListServers)
	mux.HandleFunc("POST /api/servers", s.handleCreateServer)
	mux.HandleFunc("GET /api/servers/{id}", s.handleGetServer)
	mux.HandleFunc("PATCH /api/servers/{id}", s.handleUpdateServer)
	mux.HandleFunc("DELETE /api/servers/{id}", s.handleDeleteServer)

	mux.HandleFunc("POST /api/servers/{id}/connect", s.handleConnect)
	mux.HandleFunc("POST /api/servers/{id}/disconnect", s.handleDisconnect)
	mux.HandleFunc("POST /api/servers/{id}/execute", s.handleExecute)
	mux.HandleFunc("GET /api/servers/{id}/history", s.handleHistory)

	mux.HandleFunc("GET /api/macros", s.handleListMacros)
	mux.HandleFunc("POST /api/macros", s.handleCreateMacro)
	mux.HandleFunc("DELETE /api/macros/{id}", s.handleDeleteMacro)

	mux.HandleFunc("GET /api/events", s.handleEvents)

	if s.static != nil {
		mux.Handle("/", s.spaHandler())
	}
	return s.withAuth(mux)
}

// withAuth enforces the bearer token when one is configured.
//
// The token may arrive either as an Authorization header or as an access_token
// query parameter. The query form is not laziness: a browser's EventSource
// cannot set custom headers, so a header-only scheme would make the event
// stream unauthenticatable from the very client this serves.
//
// Only /api/ is protected. The frontend bundle is served unauthenticated, which
// is a deliberate boundary rather than an oversight:
//
//   - A browser cannot attach a token to subresource requests. The document is
//     reached at /?access_token=…, but the <script> and <link> it pulls in carry
//     nothing, so gating them returns 401 and the application never starts. This
//     is not hypothetical -- it made the desktop build load a blank window.
//   - There is nothing to protect. index.html and the hashed assets are the same
//     public build shipped in every release; they contain no credentials and no
//     user data.
//
// The surface that matters -- every server profile, command and event -- is
// under /api/ and stays behind the token.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		supplied := r.URL.Query().Get("access_token")
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			supplied = strings.TrimPrefix(h, "Bearer ")
		}
		if supplied != s.token {
			writeError(w, http.StatusUnauthorized, errors.New("invalid or missing token"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleProtocols(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, rcon.Protocols())
}

// serverResponse pairs a stored profile with its live connection state, so the
// UI can render the sidebar from a single request.
type serverResponse struct {
	store.Server
	Status session.Status `json:"status"`
}

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.ListServers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	out := make([]serverResponse, 0, len(servers))
	for _, srv := range servers {
		out = append(out, serverResponse{Server: srv, Status: s.mgr.Status(srv.ID)})
	}
	writeJSON(w, http.StatusOK, out)
}

// serverRequest is the create/update payload. Password is a pointer so that
// "absent" and "set to empty" stay distinguishable: an edit that omits it must
// leave the stored password alone rather than wiping it.
type serverRequest struct {
	Name     string  `json:"name"`
	Protocol string  `json:"protocol"`
	Addr     string  `json:"addr"`
	Group    string  `json:"group"`
	Password *string `json:"password"`
}

func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var req serverRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Password == nil {
		writeError(w, http.StatusBadRequest, errors.New("password is required"))
		return
	}

	srv, err := s.store.CreateServer(r.Context(), store.Server{
		Name:     req.Name,
		Protocol: req.Protocol,
		Addr:     req.Addr,
		Group:    req.Group,
	}, *req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, serverResponse{Server: srv, Status: s.mgr.Status(srv.ID)})
}

func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	srv, err := s.store.GetServer(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, serverResponse{Server: srv, Status: s.mgr.Status(srv.ID)})
}

func (s *Server) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	var req serverRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	id := r.PathValue("id")
	srv, err := s.store.UpdateServer(r.Context(), id, store.Server{
		Name:     req.Name,
		Protocol: req.Protocol,
		Addr:     req.Addr,
		Group:    req.Group,
	}, req.Password)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	// Connection parameters may have changed, so a live session is now stale.
	// Restarting also clears a terminal auth_failed state, which is how a
	// corrected password takes effect.
	if s.mgr.Status(id) != session.StatusDisconnected {
		_ = s.mgr.Connect(id)
	}
	writeJSON(w, http.StatusOK, serverResponse{Server: srv, Status: s.mgr.Status(id)})
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mgr.Disconnect(id)

	if err := s.store.DeleteServer(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetServer(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	if err := s.mgr.Connect(id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": s.mgr.Status(id)})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	s.mgr.Disconnect(r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]any{"status": session.StatusDisconnected})
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Command) == "" {
		writeError(w, http.StatusBadRequest, errors.New("command is required"))
		return
	}

	resp, err := s.mgr.Execute(r.Context(), r.PathValue("id"), req.Command)
	if err != nil {
		// A rejected password is a client-side problem to fix, not a server
		// fault, so it must not read as a 500 in the UI.
		status := http.StatusBadGateway
		if errors.Is(err, session.ErrAuthFailed) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"response": resp})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	entries, err := s.store.ListHistory(r.Context(), r.PathValue("id"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleListMacros(w http.ResponseWriter, r *http.Request) {
	macros, err := s.store.ListMacros(r.Context(), r.URL.Query().Get("serverId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, macros)
}

func (s *Server) handleCreateMacro(w http.ResponseWriter, r *http.Request) {
	var m store.Macro
	if err := decodeJSON(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	created, err := s.store.CreateMacro(r.Context(), m)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDeleteMacro(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteMacro(r.Context(), r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// sseHeartbeat keeps idle connections alive. Proxies and load balancers commonly
// close a stream that has been silent for 60s, and an RCON console is idle most
// of the time.
const sseHeartbeat = 25 * time.Second

// handleEvents streams events as SSE.
//
// Supports ?profileId= to filter to one server, and resumption via the
// Last-Event-ID header or ?lastEventId=, which replays anything retained since
// that sequence number.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}

	profileID := r.URL.Query().Get("profileId")

	var lastSeq uint64
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		lastSeq, _ = strconv.ParseUint(v, 10, 64)
	} else if v := r.URL.Query().Get("lastEventId"); v != "" {
		lastSeq, _ = strconv.ParseUint(v, 10, 64)
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Tells nginx not to buffer, which would otherwise defeat streaming
	// entirely behind a typical reverse proxy.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Subscribe before replaying history, so an event published during the
	// replay is queued rather than lost in the gap between the two.
	events, unsubscribe := s.hub.Subscribe(profileID)
	defer unsubscribe()

	replayed := lastSeq
	for _, e := range s.hub.History(profileID, lastSeq) {
		writeSSE(w, e)
		replayed = e.Seq
	}
	flusher.Flush()

	ticker := time.NewTicker(sseHeartbeat)
	defer ticker.Stop()

	for {
		select {
		case e, ok := <-events:
			if !ok {
				return
			}
			// Skip anything the replay already delivered.
			if e.Seq <= replayed {
				continue
			}
			writeSSE(w, e)
			flusher.Flush()

		case <-ticker.C:
			// An SSE comment: ignored by EventSource, but traffic on the wire.
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

func writeSSE(w http.ResponseWriter, e event.Event) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	// id: lets the browser resume via Last-Event-ID after a dropped connection.
	fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Seq, e.Stream, data)
}

// spaHandler serves the embedded frontend, falling back to index.html so
// client-side routes survive a page reload.
func (s *Server) spaHandler() http.Handler {
	files := http.FileServer(http.FS(s.static))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(s.static, path); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeError(w, http.StatusInternalServerError, err)
}
