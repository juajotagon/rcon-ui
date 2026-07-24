// Package config resolves runtime settings from a file, the environment and
// command-line flags, in that order of increasing precedence.
//
// Anything settable in the UI must also be settable here: for a self-hosted
// tool, a config file is how the deployment is reproduced, and settings that
// exist only behind a UI toggle cannot be checked into a Git repository or
// managed by ArgoCD.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	// Addr is the listen address. Defaults to loopback: a tool holding RCON
	// passwords should not become reachable from the network because someone
	// skipped reading the docs. Exposing it is a deliberate act.
	Addr string `json:"addr"`

	// DataDir holds the database and, in local mode, the sealing key.
	DataDir string `json:"dataDir"`

	// Token, when set, is required as a bearer token on every API request.
	// The desktop shell generates one per launch; a server deployment should
	// set one whenever Addr is not loopback.
	Token string `json:"token"`

	// LogLevel is debug, info, warn or error.
	LogLevel string `json:"logLevel"`
}

func Default() Config {
	return Config{
		Addr:     "127.0.0.1:8477",
		DataDir:  defaultDataDir(),
		LogLevel: "info",
	}
}

func defaultDataDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "rcon-ui")
	}
	return ".rcon-ui"
}

// DBPath is where the SQLite database lives.
func (c Config) DBPath() string { return filepath.Join(c.DataDir, "rcon-ui.db") }

// Load reads the config file if present, then applies environment overrides.
// A missing file is not an error; a malformed one is, because silently ignoring
// a config the user believed was in effect is worse than refusing to start.
func Load(path string) (Config, error) {
	cfg := Default()

	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := json.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("config: parse %s: %w", path, err)
			}
		case errors.Is(err, os.ErrNotExist):
			// Fine: defaults plus environment.
		default:
			return cfg, fmt.Errorf("config: read %s: %w", path, err)
		}
	}

	if v := os.Getenv("RCON_UI_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("RCON_UI_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("RCON_UI_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("RCON_UI_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	return cfg, nil
}

// DefaultConfigPath is where Load looks when no path is given.
func DefaultConfigPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "rcon-ui", "config.json")
	}
	return "rcon-ui.json"
}

// IsLoopback reports whether Addr binds only to the local machine. Used to warn
// when the API is exposed to a network without a token.
func IsLoopback(addr string) bool {
	host, _, err := splitHostPort(addr)
	if err != nil {
		return false
	}
	switch host {
	case "127.0.0.1", "localhost", "::1", "[::1]":
		return true
	default:
		return false
	}
}

// splitHostPort tolerates a bare ":8477", which means every interface.
func splitHostPort(addr string) (host, port string, err error) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			host, port = addr[:i], addr[i+1:]
			if _, convErr := strconv.Atoi(port); convErr != nil {
				return "", "", fmt.Errorf("config: bad port in %q", addr)
			}
			return host, port, nil
		}
	}
	return "", "", fmt.Errorf("config: address %q has no port", addr)
}
