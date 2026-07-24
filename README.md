# rcon-ui

A protocol-agnostic RCON client for game servers, with a modern UI. Self-host it
as a single static binary, or run it as a desktop app.

> **Status: early.** The protocol core and a validation CLI work. There is no UI
> yet. See [Roadmap](#roadmap).

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

Requires Go 1.24+. No other dependencies — the daemon builds with `CGO_ENABLED=0`
and cross-compiles to Linux, Windows and macOS from any host.

```sh
make build

# The password is read from $RCON_PASSWORD. Passing -password puts it in your
# shell history and in the process list.
RCON_PASSWORD=secret ./bin/rcon-ui connect mc.internal:25575

# Or run a single command and exit:
RCON_PASSWORD=secret ./bin/rcon-ui connect mc.internal:25575 -c list
```

`connect` is a temporary REPL for validating the protocol against real servers.
It will be replaced by the daemon and UI.

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
- [ ] Server profiles, sealed credentials, session manager, HTTP/WebSocket API
- [ ] Web UI (embedded in the binary)
- [ ] Desktop app (Wails)
- [ ] Container image, Helm chart, signed release builds
- [ ] Optional log streaming (Kubernetes pod logs, file tail, `docker logs`)

## License

Not yet chosen.
