// Command rcon-ui-desktop is the desktop shell.
//
// It is deliberately thin. Rather than exposing Wails bindings to the
// frontend, it starts the same HTTP server the daemon runs -- bound to
// loopback on an ephemeral port -- and points a webview at it. The frontend
// therefore cannot tell which shell it is running in, there is exactly one
// transport, and every feature works identically in both builds. Anything that
// reaches for a Wails-only binding should be questioned: it would fork the two
// shells and reintroduce the dual-transport problem this avoids.
//
// Requires CGO and the platform's webview development headers, so unlike the
// daemon it does not cross-compile. Release builds use a per-OS runner matrix.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/juajotagon/rcon-ui/internal/api"
	"github.com/juajotagon/rcon-ui/internal/config"
	"github.com/juajotagon/rcon-ui/internal/event"
	"github.com/juajotagon/rcon-ui/internal/secret"
	"github.com/juajotagon/rcon-ui/internal/session"
	"github.com/juajotagon/rcon-ui/internal/store"
	"github.com/juajotagon/rcon-ui/internal/webui"

	_ "github.com/juajotagon/rcon-ui/internal/rcon/source"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatalf("rcon-ui-desktop: %v", err)
	}
}

func run() error {
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		return err
	}

	// Honours RCON_UI_LOG_LEVEL like the daemon. accessLog below logs at debug
	// level, so request tracing is opt-in rather than on in every release.
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	key, _, err := secret.LoadOrCreateKey(cfg.DataDir)
	if err != nil {
		return err
	}
	sealer, err := secret.NewFromKey(key)
	if err != nil {
		return err
	}

	st, err := store.Open(cfg.DBPath(), sealer)
	if err != nil {
		return err
	}
	defer st.Close()

	hub := event.NewHub()
	mgr := session.NewManager(st, hub, log)
	defer mgr.Close()

	// A per-launch token, not a fixed one. Binding to loopback is not by itself
	// access control: any other local process can reach the port, and a web
	// page in the user's ordinary browser can too via DNS rebinding. Without
	// this, visiting a hostile site while the app is open would hand that site
	// full RCON control of every configured server.
	token, err := newToken()
	if err != nil {
		return err
	}

	static, ok := webui.FS()
	if !ok {
		return fmt.Errorf("no UI embedded in this build; run `make ui` first")
	}

	// Port 0: the OS picks a free port. A fixed one would collide with a
	// running daemon and makes the surface predictable.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen on loopback: %w", err)
	}

	srv := &http.Server{
		Handler:           accessLog(log, api.New(st, hub, mgr, log, token, static).Handler()),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: it would sever the SSE stream, which stays open for
		// the lifetime of the window.
		IdleTimeout: 120 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("local server stopped", "error", err)
		}
	}()
	defer srv.Shutdown(context.Background())

	// The token rides in the query string because a webview cannot be given
	// custom request headers for its initial navigation. The frontend reads it
	// once and strips it from the visible URL.
	target := fmt.Sprintf("http://%s/?access_token=%s", ln.Addr().String(), token)
	log.Info("desktop shell starting", "addr", ln.Addr().String(), "version", version)

	return wails.Run(&options.App{
		Title:  "rcon-ui",
		Width:  1200,
		Height: 800,

		MinWidth:  760,
		MinHeight: 480,

		BackgroundColour: &options.RGBA{R: 22, G: 24, B: 29, A: 255},

		// Quit on SIGINT/SIGTERM. Without a handler the process ignores a plain
		// `kill` -- Wails installs none of its own -- so the only way to stop it
		// is `kill -9`. That is especially bad under a compositor that draws no
		// window decorations (niri and other tiling Wayland compositors give the
		// window no titlebar and so no close button), where there is otherwise no
		// way out at all. OnStartup is where the Wails runtime context first
		// becomes available, which runtime.Quit needs.
		OnStartup: func(ctx context.Context) {
			go func() {
				sig := make(chan os.Signal, 1)
				signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
				s := <-sig
				log.Info("received signal, shutting down", "signal", s.String())
				wailsruntime.Quit(ctx)
			}()
		},

		// Wails serves only a bootstrap page, which immediately redirects to
		// the loopback server above.
		//
		// The obvious alternative -- handing the API straight to
		// AssetServer.Handler and skipping the HTTP listener -- does not work:
		// Wails' webview ResponseWriters implement no Flush, so a Server-Sent
		// Events response never reaches the page and the live console silently
		// stays empty. Redirecting makes the webview perform a real HTTP
		// navigation, where streaming is the browser engine's own job.
		AssetServer: &assetserver.Options{
			Handler: bootstrapHandler(target),
		},

		Linux: &linux.Options{
			ProgramName: "rcon-ui",

			// Wails defaults this to WebviewGpuPolicyNever on Linux (see
			// wails#2977), which disables hardware acceleration entirely and is
			// a large part of why WebKitGTK apps are reported as sluggish
			// there. OnDemand restores acceleration while still letting the
			// driver decline, which is the safer of the accelerated settings on
			// the NVIDIA/DMABUF combination that causes the known problems.
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
		},
	})
}

// accessLog records requests reaching the loopback server.
//
// Worth keeping rather than deleting after debugging: the webview is opaque --
// there is no devtools network tab in a production Wails build -- so "did the
// page actually load" is otherwise unanswerable. Silence here means the webview
// never navigated, which is a completely different fault from a page that
// loaded and then failed.
func accessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// Logging the status, not just the request, is the point. An earlier
		// version logged before calling the handler, so a 401 on the asset
		// bundle looked exactly like a successful load -- the page appeared to
		// fetch its JavaScript while actually receiving nothing.
		log.Debug("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"query", redactToken(r.URL.RawQuery))
	})
}

// statusRecorder captures the response status for logging.
//
// It forwards Flush, without which the SSE stream would be buffered here and
// the live console would never update -- the exact failure this wrapper exists
// to make visible.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// redactToken keeps the session token out of the log while leaving other
// query parameters (notably diagnostic messages) readable.
func redactToken(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := neturl.ParseQuery(raw)
	if err != nil {
		return "<unparseable>"
	}
	if values.Has("access_token") {
		values.Set("access_token", "<redacted>")
	}
	return values.Encode()
}

// bootstrapHandler serves a minimal page that redirects the webview to the
// loopback server. It never needs streaming, so Wails' non-flushing writer is
// fine here.
func bootstrapHandler(target string) http.Handler {
	page := fmt.Sprintf(`<!doctype html>
<meta charset="utf-8">
<title>rcon-ui</title>
<body style="margin:0;background:#16181d"></body>
<script>location.replace(%q)</script>
`, target)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, page)
	})
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
