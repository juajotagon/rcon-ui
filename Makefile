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

.PHONY: build
build:
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BIN)/rcon-ui ./cmd/rcon-ui

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
.PHONY: crosscheck
crosscheck:
	GOOS=windows GOARCH=amd64 $(GO) build -o /dev/null ./cmd/rcon-ui
	GOOS=linux   GOARCH=amd64 $(GO) build -o /dev/null ./cmd/rcon-ui
	GOOS=linux   GOARCH=arm64 $(GO) build -o /dev/null ./cmd/rcon-ui
	GOOS=darwin  GOARCH=arm64 $(GO) build -o /dev/null ./cmd/rcon-ui

.PHONY: clean
clean:
	rm -rf $(BIN) dist
