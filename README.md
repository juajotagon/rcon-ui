# rcon-ui

A protocol-agnostic RCON client for game servers. A native desktop app for
Linux and Windows, which can also run headless as a single static binary.

> **Status: working, unreleased.** The app runs and does what it says. There are
> no packaged downloads yet, so you build it yourself. See [Roadmap](#roadmap).

## What this is

Connect to your game servers, send RCON commands, see the responses — without
memorising `docker exec` incantations or juggling `mcrcon` invocations. Manage
several servers from one place.

Every instance is **single-user**. There are no accounts, no permissions and no
shared state: you run your own, your friends run theirs. That is a deliberate
simplification, not a missing feature.

## What this is *not*

**It cannot show you a live server log.** Source RCON is strictly
request/response — the protocol has no mechanism for a server to push its
console to a connected client. The console shows *the commands you sent and
their replies*, nothing more.

This is the most common wrong expectation people bring to an RCON tool, so it is
stated plainly here. Real log streaming needs access to the game host's log
files or container, which is a different capability from RCON; it is on the
roadmap as an optional per-server addition, not a requirement.

## Try it

Requires Go 1.25+ and Node 20+.

```sh
# Desktop app — the normal way to run this.
make desktop && ./bin/rcon-ui-desktop
```

The desktop build needs your platform's webview development headers
(`webkit2gtk-4.1` and GTK 3 on Linux, WebView2 on Windows). It starts the same
server the daemon runs, on a random loopback port behind a per-launch token, and
points a native window at it — so there is one codebase and one transport, not a
desktop fork.

### Headless

The same binary can run without a window, which is useful on a box you reach
over the network:

```sh
make build

# Binds to 127.0.0.1:8477 by default.
./bin/rcon-ui serve
```

Then drive it over HTTP:

```sh
# Add a server
curl -X POST localhost:8477/api/servers -H 'Content-Type: application/json' \
  -d '{"name":"Minecraft","addr":"mc.internal:25575","password":"secret"}'

# Send a command
curl -X POST localhost:8477/api/servers/<id>/execute \
  -H 'Content-Type: application/json' -d '{"command":"list"}'

# Watch the live event stream
curl -N localhost:8477/api/events
```

There is also a one-off REPL, kept for validating the protocol against real
servers:

```sh
# The password is read from $RCON_PASSWORD. Passing -password puts it in your
# shell history and in the process list.
RCON_PASSWORD=secret ./bin/rcon-ui connect mc.internal:25575 -c list
```

### Configuration

Settings resolve from a config file, then environment, then flags. Anything
settable in the UI is settable in the file, so a deployment can be reproduced
from Git.

| Flag | Environment | Default |
| --- | --- | --- |
| `-addr` | `RCON_UI_ADDR` | `127.0.0.1:8477` |
| `-data-dir` | `RCON_UI_DATA_DIR` | OS config dir |
| `-token` | `RCON_UI_TOKEN` | none |
| — | `RCON_UI_KEY` | generated locally |

Two things worth understanding before exposing this beyond localhost:

- **`RCON_UI_KEY` seals stored passwords.** Without it, a key is generated
  *beside* the database, which is fine for a desktop install where both are
  already behind your user account. On a shared or remote host it is not: anyone
  who obtains the data directory obtains the passwords with it, so set the key
  from the environment there.
- **`-token` is the only authentication.** Binding to anything other than
  loopback without one hands RCON access to whoever can reach the port. The
  daemon warns at startup if you do.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/servers` | list profiles with live status |
| `POST` | `/api/servers` | add a profile |
| `PATCH`/`DELETE` | `/api/servers/{id}` | edit / remove |
| `POST` | `/api/servers/{id}/connect` | open a session |
| `POST` | `/api/servers/{id}/execute` | run a command |
| `GET` | `/api/servers/{id}/history` | recent commands |
| `GET` | `/api/servers/{id}/discovery` | cached fingerprint + capabilities report |
| `POST` | `/api/servers/{id}/discover` | re-run discovery against the live session |
| `GET`/`POST` | `/api/macros` | saved commands |
| `GET` | `/api/events` | live event stream (SSE) |

Streaming is Server-Sent Events rather than WebSocket. The flow is strictly
server-to-client — commands go the other way as plain POSTs — so SSE fits the
shape exactly using only `net/http`, and `EventSource` provides reconnection and
`Last-Event-ID` resumption without hand-written code.

Every streamed frame is the same envelope, whatever produced it:

```json
{"seq":4,"profileId":"961f…","source":"rcon","stream":"response",
 "line":"There are 3 of a max of 20 players online","at":"2026-07-24T01:29:26Z"}
```

A command and its reply are **two** events, not one paired object. That is what
lets a future log source — which has no command half — join the same stream
without changing the protocol or the console.

## Supported protocols

| Protocol | Status | Games |
| --- | --- | --- |
| Source RCON (TCP) | supported | Minecraft, Project Zomboid, CS2, Rust, ARK, Palworld, Valheim, 7DTD, Squad |
| BattlEye (UDP) | planned | Arma 3, DayZ |
| Quake / GameSpy (UDP) | planned | older Call of Duty, Quake-engine games |

"RCON" is not one protocol. Adding a dialect means writing a new package that
implements `rcon.Dialer` and calling `rcon.Register` — no changes to callers.

## Development

```sh
make test        # unit tests; needs no game server
make race        # race detector (uses CGO, unlike the shipped binary)
make vet
make crosscheck  # proves the daemon still cross-compiles with CGO disabled
```

The test suite includes a fake RCON server, so CI never needs a real game
server. It covers the parts that are easy to get subtly wrong: auth failure
signalled by `id == -1`, reassembly of replies split across packets, Valve's
double-sentinel quirk, and request/response correlation under concurrency.

Round-trip tests alone are not enough — `decode(encode(p)) == p` holds even when
the wire format is wrong, because the decoder is wrong in the same way, and the
fake server replicates the same misreading. `golden_test.go` therefore pins the
exact bytes, hand-computed from the published field table rather than generated
by our own encoder. Swapping the id and type fields makes every other test in
the package still pass; the golden vectors fail.

Even so, a fake server can only prove the client matches our reading of the
protocol. Changes to the wire format should also be smoke-tested against a real
server.

## Roadmap

- [x] Protocol core + registry, Source RCON, validation CLI
- [x] Server profiles, sealed credentials, session manager, HTTP + SSE API
- [x] Web UI (embedded in the binary)
- [x] Desktop app (Wails)
- [ ] Signed release builds for Linux and Windows
- [ ] Optional log streaming (file tail, `docker logs`, Kubernetes pod logs)

## License

Not yet chosen.
