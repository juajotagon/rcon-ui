# The daemon must stay CGO-free: the "one static binary, cross-compiles
# anywhere" deploy story depends on it. CGO_ENABLED=0 is set on every daemon
# target so a CGO-requiring dependency (e.g. mattn/go-sqlite3) fails here
# rather than at release time. The Wails desktop shell is the deliberate
# exception -- it links native webview libraries and always needs CGO.
export CGO_ENABLED = 0

GO      ?= go
BIN     ?= bin
PKG     := github.com/juajotagon/rcon-ui
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all
all: test build

# Builds the frontend into internal/webui/dist, where go:embed picks it up.
.PHONY: ui
ui:
	cd web && npm ci --prefer-offline --no-audit --fund=false && npm run build
	@# Vite empties the output directory, taking the committed placeholder with
	@# it. Restoring it keeps a later `git clean` or fresh checkout compiling.
	@printf '%s\n' \
		'Placeholder so `go build` works on a clone where the UI has not been built.' \
		'Run `make ui` (or `make build`) to generate the real frontend here.' \
		> internal/webui/dist/.gitkeep

# build assumes the frontend is already built; use `make dist` for both. Kept
# separate so the Go/UI edit loops stay independent -- rebuilding the whole
# frontend to test a Go change would be a slow default.
.PHONY: build
build:
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BIN)/rcon-ui ./cmd/rcon-ui

# Full artifact: frontend embedded in the binary.
.PHONY: dist
dist: ui build

.PHONY: test
test:
	$(GO) test ./...

.PHONY: race
race:
	CGO_ENABLED=1 $(GO) test -race ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: tidy
tidy:
	$(GO) mod tidy

# Proves the cross-compile guarantee holds. Kept green from Phase 0 onward so a
# Windows-hostile or CGO-requiring dependency is caught the day it lands.
#
# Builds every daemon package, not just the command: a dependency that fails to
# cross-compile usually enters through a library (the SQLite driver being the
# obvious candidate), and would go unnoticed until something imported it.
#
# The desktop shell is excluded because it genuinely cannot cross-compile --
# Wails links the target OS's native webview headers. That is a real constraint,
# not a workaround, and it is why release builds use a per-OS runner matrix.
# Excluding it here keeps this target meaningful for the half that must stay
# portable.
DAEMON_PKGS = $(shell $(GO) list ./... | grep -v '/cmd/rcon-ui-desktop')

.PHONY: crosscheck
crosscheck:
	GOOS=windows GOARCH=amd64 $(GO) build $(DAEMON_PKGS)
	GOOS=linux   GOARCH=amd64 $(GO) build $(DAEMON_PKGS)
	GOOS=linux   GOARCH=arm64 $(GO) build $(DAEMON_PKGS)
	GOOS=darwin  GOARCH=arm64 $(GO) build $(DAEMON_PKGS)

# Desktop shell. Needs CGO and the platform's webview development headers
# (webkit2gtk-4.1 + gtk3 on Linux, WebView2 on Windows), so it only builds for
# the host OS.
#
# The `desktop,production` tags are required: Wails selects its webview
# implementation through build tags and refuses to start without them. The
# wails CLI normally injects these, and plain `go build` produces a binary that
# compiles but exits immediately at runtime.
#
# WEBKIT_TAG picks the WebKitGTK API version. Wails defaults to webkit2gtk-4.0,
# which current distributions have replaced with 4.1 (the libsoup3 build), so
# the default fails with "Package 'webkit2gtk-4.0' not found". Detected rather
# than hard-coded, because both are still in the wild.
WEBKIT_TAG := $(shell pkg-config --exists webkit2gtk-4.1 && echo webkit2_41 || echo webkit2_40)

.PHONY: desktop
desktop: ui
	@echo "building desktop shell with $(WEBKIT_TAG)"
	CGO_ENABLED=1 $(GO) build -tags desktop,production,$(WEBKIT_TAG) -ldflags '$(LDFLAGS)' \
		-o $(BIN)/rcon-ui-desktop ./cmd/rcon-ui-desktop

.PHONY: clean
clean:
	rm -rf $(BIN) dist
