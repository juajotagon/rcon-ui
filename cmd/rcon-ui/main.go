// Command rcon-ui is the rcon-ui daemon.
//
// Today it only exposes `connect`, a minimal REPL used to validate the
// protocol implementation against real game servers before any UI exists. The
// HTTP/WebSocket server arrives in Phase 2.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/juajotagon/rcon-ui/internal/rcon"

	// Registers the Source dialect. Future dialects are added the same way.
	_ "github.com/juajotagon/rcon-ui/internal/rcon/source"
)

// version is set at build time via -ldflags. See the Makefile.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "rcon-ui: %v\n", err)
		if errors.Is(err, rcon.ErrAuthFailed) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("no command given")
	}

	switch args[0] {
	case "serve":
		return cmdServe(args[1:])
	case "connect":
		return cmdConnect(args[1:])
	case "protocols":
		for _, p := range rcon.Protocols() {
			fmt.Println(p)
		}
		return nil
	case "version", "--version", "-v":
		fmt.Println(version)
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `rcon-ui - RCON client

Usage:
  rcon-ui serve [flags]                 run the daemon and web API
  rcon-ui connect <host:port> [flags]   one-off REPL against a server
  rcon-ui protocols                     list registered RCON dialects
  rcon-ui version                       print the version

Serve flags:
  -addr string       listen address (default 127.0.0.1:8477)
  -config string     path to config file
  -data-dir string   data directory
  -token string      require this bearer token

Connect flags:
  -password string   RCON password (prefer $RCON_PASSWORD)
  -protocol string   RCON dialect (default "source")
  -timeout duration  per-command timeout (default 10s)
  -c string          run a single command and exit
  -tls               wrap the connection in TLS (for TLS-terminating proxies)
  -servername string TLS server name / SNI (defaults to the host in <host:port>)

The password is read from $RCON_PASSWORD when -password is omitted. Passing it
as a flag puts it in your shell history and in the process list, so prefer the
environment variable.

Examples:
  RCON_PASSWORD=secret rcon-ui connect mc.internal:25575
  RCON_PASSWORD=secret rcon-ui connect mc.internal:25575 -c list
`)
}

// parseInterspersed parses flags that may appear before or after positional
// arguments. The standard flag package stops at the first non-flag token, so
// `connect host:port -c list` would otherwise silently ignore -c and fail with
// a confusing argument-count error.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string

	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) > 0 {
			positional = append(positional, args[0])
			args = args[1:]
		}
	}
	return positional, nil
}

func cmdConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	password := fs.String("password", "", "RCON password (prefer $RCON_PASSWORD)")
	protocol := fs.String("protocol", rcon.ProtocolSource, "RCON dialect")
	timeout := fs.Duration("timeout", rcon.DefaultTimeout, "per-command timeout")
	oneShot := fs.String("c", "", "run a single command and exit")
	useTLS := fs.Bool("tls", false, "wrap the connection in TLS (for TLS-terminating proxies)")
	serverName := fs.String("servername", "", "TLS server name / SNI (defaults to the host in <host:port>)")

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		return fmt.Errorf("connect requires exactly one <host:port> argument, got %d", len(positional))
	}
	addr := positional[0]

	if *password == "" {
		*password = os.Getenv("RCON_PASSWORD")
	}
	if *password == "" {
		return errors.New("no password: set $RCON_PASSWORD or pass -password")
	}

	// Ctrl-C cancels in-flight commands and unwinds cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dialCtx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	client, err := rcon.Dial(dialCtx, *protocol, rcon.Target{
		Addr:       addr,
		Password:   *password,
		Timeout:    *timeout,
		TLS:        *useTLS,
		ServerName: *serverName,
	})
	if err != nil {
		return err
	}
	defer client.Close()

	if *oneShot != "" {
		return execAndPrint(ctx, client, *oneShot)
	}
	return repl(ctx, client, addr)
}

func repl(ctx context.Context, client rcon.Client, addr string) error {
	fmt.Printf("connected to %s -- type a command, or Ctrl-D to quit\n", addr)
	fmt.Println("note: RCON is request/response; this shows your commands and their")
	fmt.Println("      replies, not a live feed of the server log.")

	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("rcon> ")

		if !in.Scan() {
			if err := in.Err(); err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			fmt.Println()
			return nil // Ctrl-D
		}

		cmd := strings.TrimSpace(in.Text())
		switch cmd {
		case "":
			continue
		case "exit", "quit":
			return nil
		}

		if err := execAndPrint(ctx, client, cmd); err != nil {
			// Keep the REPL alive on a per-command failure, but a broken
			// stream will fail every subsequent command -- that is the signal
			// to reconnect, which Phase 2's session manager will automate.
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			if ctx.Err() != nil {
				return nil
			}
		}
	}
}

func execAndPrint(ctx context.Context, client rcon.Client, cmd string) error {
	start := time.Now()
	resp, err := client.Execute(ctx, cmd)
	if err != nil {
		return err
	}

	if resp == "" {
		fmt.Printf("(empty response, %s)\n", time.Since(start).Round(time.Millisecond))
		return nil
	}
	fmt.Println(strings.TrimRight(resp, "\n"))
	return nil
}
