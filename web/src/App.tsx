import { useCallback, useEffect, useMemo, useState } from "react";
import { api, type Macro, type Server } from "./api";
import { useEvents } from "./useEvents";
import { Console } from "./components/Console";
import { Sidebar } from "./components/Sidebar";
import { CommandInput } from "./components/CommandInput";
import { ServerDialog } from "./components/ServerDialog";
import { CommandPalette, type Command } from "./components/CommandPalette";

const SELECTED_KEY = "rcon-ui:selected";

export function App() {
  const [servers, setServers] = useState<Server[]>([]);
  const [protocols, setProtocols] = useState<string[]>(["source"]);
  const [selectedId, setSelectedId] = useState<string | null>(
    () => localStorage.getItem(SELECTED_KEY),
  );
  const [macros, setMacros] = useState<Macro[]>([]);
  const [history, setHistory] = useState<string[]>([]);
  const [dialog, setDialog] = useState<{ open: boolean; server: Server | null }>({
    open: false,
    server: null,
  });
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const events = useEvents(selectedId);
  const selected = servers.find((s) => s.id === selectedId) ?? null;

  const refreshServers = useCallback(async () => {
    try {
      const list = await api.listServers();
      setServers(list);
      setLoadError(null);

      // Drop a selection whose server no longer exists, rather than leaving the
      // UI pointing at nothing.
      setSelectedId((current) =>
        current && list.some((s) => s.id === current) ? current : (list[0]?.id ?? null),
      );
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    refreshServers();
    api.protocols().then(setProtocols).catch(() => {});
  }, [refreshServers]);

  // Status arrives over the event stream, but the sidebar renders from the
  // server list, so a status event is the cue to refetch it.
  useEffect(() => {
    if (events.length === 0) return;
    if (events[events.length - 1]?.stream === "status") refreshServers();
  }, [events, refreshServers]);

  useEffect(() => {
    if (!selectedId) {
      setMacros([]);
      setHistory([]);
      return;
    }
    localStorage.setItem(SELECTED_KEY, selectedId);
    api.listMacros(selectedId).then(setMacros).catch(() => setMacros([]));
    api
      .history(selectedId)
      .then((h) => setHistory(h.map((e) => e.command)))
      .catch(() => setHistory([]));
  }, [selectedId]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const execute = async (command: string) => {
    if (!selectedId) return;
    // Prepend locally so ↑ finds it immediately; the daemon records it too, and
    // the next fetch is authoritative.
    setHistory((h) => [command, ...h.filter((c) => c !== command)]);
    try {
      await api.execute(selectedId, command);
    } catch {
      // The failure is already on the event stream as an error line; surfacing
      // it twice would just be noise.
    }
  };

  const commands = useMemo<Command[]>(() => {
    const list: Command[] = [
      { id: "add", label: "Add server…", run: () => setDialog({ open: true, server: null }) },
      { id: "theme", label: "Toggle light / dark", run: () => document.documentElement.classList.toggle("dark") },
    ];

    if (selected) {
      list.push(
        { id: "edit", label: `Edit “${selected.name}”…`, run: () => setDialog({ open: true, server: selected }) },
        { id: "connect", label: `Reconnect “${selected.name}”`, run: () => api.connect(selected.id).then(refreshServers) },
        { id: "disconnect", label: `Disconnect “${selected.name}”`, run: () => api.disconnect(selected.id).then(refreshServers) },
        {
          id: "delete",
          label: `Delete “${selected.name}”…`,
          run: () => {
            if (confirm(`Delete “${selected.name}”? This cannot be undone.`)) {
              api.deleteServer(selected.id).then(refreshServers);
            }
          },
        },
      );
      for (const m of macros) {
        list.push({ id: `macro-${m.id}`, label: m.name, hint: m.command, run: () => execute(m.command) });
      }
      for (const cmd of history.slice(0, 20)) {
        list.push({ id: `history-${cmd}`, label: cmd, hint: "recent", run: () => execute(cmd) });
      }
    }
    return list;
  }, [selected, macros, history, refreshServers]);

  const servers_ = servers;

  return (
    <div className="flex h-full">
      <Sidebar
        servers={servers_}
        selectedId={selectedId}
        onSelect={setSelectedId}
        onAdd={() => setDialog({ open: true, server: null })}
        onOpenPalette={() => setPaletteOpen(true)}
      />

      <main className="flex min-w-0 flex-1 flex-col">
        {loadError ? (
          <Centered>
            <p className="text-sm" style={{ color: "var(--color-danger)" }}>
              Cannot reach the rcon-ui daemon.
            </p>
            <p className="mt-1 font-mono text-xs opacity-70">{loadError}</p>
          </Centered>
        ) : !selected ? (
          <Centered>
            <p className="text-sm font-medium">No server selected</p>
            <p className="mt-1 text-xs opacity-70">
              Add a server to get started, or press <kbd className="rounded border px-1">⌘K</kbd>.
            </p>
          </Centered>
        ) : (
          <>
            <header className="flex items-center gap-3 border-b px-4 py-2.5">
              <div className="min-w-0">
                <h1 className="truncate text-sm font-semibold">{selected.name}</h1>
                <p className="truncate font-mono text-xs" style={{ color: "var(--color-text-muted)" }}>
                  {selected.addr} · {selected.protocol}
                </p>
              </div>

              <div className="ml-auto flex items-center gap-2">
                <StatusBadge status={selected.status} />
                {selected.status === "connected" ? (
                  <button onClick={() => api.disconnect(selected.id).then(refreshServers)} className="btn btn-ghost text-xs">
                    Disconnect
                  </button>
                ) : (
                  <button onClick={() => api.connect(selected.id).then(refreshServers)} className="btn btn-ghost text-xs">
                    Connect
                  </button>
                )}
                <button onClick={() => setDialog({ open: true, server: selected })} className="btn btn-ghost text-xs">
                  Edit
                </button>
              </div>
            </header>

            {selected.status === "auth_failed" && (
              <div
                className="border-b px-4 py-2 text-xs"
                style={{ background: "var(--color-surface-sunken)", color: "var(--color-danger)" }}
              >
                The server rejected this password. Reconnecting is not retried automatically —
                fix the password under Edit, which restarts the session.
              </div>
            )}

            <Console
              events={events}
              emptyHint={
                selected.status === "connected"
                  ? "Connected. Type a command below — RCON shows replies to what you send, not a live server log."
                  : "Not connected yet."
              }
            />

            <CommandInput
              disabled={selected.status !== "connected"}
              history={history}
              macros={macros}
              onSubmit={execute}
            />
          </>
        )}
      </main>

      {dialog.open && (
        <ServerDialog
          server={dialog.server}
          protocols={protocols}
          onClose={() => setDialog({ open: false, server: null })}
          onSaved={refreshServers}
        />
      )}

      {paletteOpen && <CommandPalette commands={commands} onClose={() => setPaletteOpen(false)} />}
    </div>
  );
}

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-1 items-center justify-center p-8 text-center">
      <div>{children}</div>
    </div>
  );
}

function StatusBadge({ status }: { status: Server["status"] }) {
  const label = {
    connected: "Connected",
    connecting: "Connecting…",
    auth_failed: "Auth failed",
    disconnected: "Disconnected",
  }[status];

  const colour = {
    connected: "var(--color-ok)",
    connecting: "var(--color-warn)",
    auth_failed: "var(--color-danger)",
    disconnected: "var(--color-text-muted)",
  }[status];

  return (
    <span className="flex items-center gap-1.5 text-xs" style={{ color: colour }}>
      <span className="size-2 rounded-full" style={{ background: colour }} />
      {label}
    </span>
  );
}
