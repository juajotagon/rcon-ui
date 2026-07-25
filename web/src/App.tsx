import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, ApiError, type DiscoveryReport, type Macro, type Server } from "./api";
import { useEvents } from "./useEvents";
import { Console } from "./components/Console";
import { Sidebar } from "./components/Sidebar";
import { CommandInput, type Prefill } from "./components/CommandInput";
import { ServerDialog } from "./components/ServerDialog";
import { CommandPalette, type Command } from "./components/CommandPalette";
import { SettingsPanel } from "./components/SettingsPanel";
import { CommandsPanel } from "./components/CommandsPanel";
import { TemplatesPanel } from "./components/TemplatesPanel";

const SELECTED_KEY = "rcon-ui:selected";

type Tab = "settings" | "commands" | "console";

// Templates is a top-level mode, not a tab -- it has to render with zero
// servers configured and without any server selected, which a tab nested
// inside the `selected`-server branch below could never do.
type View = "server" | "templates";

/** The rung of the degradation ladder a report puts a server on, in priority
 * order -- options+commands, then commands only, then raw console. Used both
 * for which tabs are shown and which one is picked by default. */
function defaultTab(report: DiscoveryReport | null): Tab {
  if (report?.capabilities.options) return "settings";
  if (report?.capabilities.commands) return "commands";
  return "console";
}

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
  const [view, setView] = useState<View>("server");

  const [report, setReport] = useState<DiscoveryReport | null>(null);
  const [reportError, setReportError] = useState<string | null>(null);
  const [rescanning, setRescanning] = useState(false);
  const [tab, setTab] = useState<Tab>("console");
  const tabTouched = useRef(false);
  const [prefill, setPrefill] = useState<Prefill | null>(null);

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

  // A 404 here means "never discovered", the normal state for a server that
  // has not connected yet -- not an error, so it resolves to a null report
  // rather than falling into reportError.
  const refreshDiscovery = useCallback(async (id: string) => {
    try {
      const r = await api.discovery(id);
      setReport(r);
      setReportError(null);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        setReport(null);
        setReportError(null);
      } else {
        setReport(null);
        setReportError(e instanceof Error ? e.message : String(e));
      }
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
    const last = events[events.length - 1];
    if (last?.stream === "status") refreshServers();
    // A discovery event only ever follows a scan this client (or another
    // client of the same daemon) triggered -- refetching here never itself
    // produces another discovery event, so there is no loop.
    if (last?.stream === "discovery" && selectedId) refreshDiscovery(selectedId);
  }, [events, refreshServers, refreshDiscovery, selectedId]);

  useEffect(() => {
    if (!selectedId) {
      setMacros([]);
      setHistory([]);
      setReport(null);
      setReportError(null);
      return;
    }
    localStorage.setItem(SELECTED_KEY, selectedId);
    api.listMacros(selectedId).then(setMacros).catch(() => setMacros([]));
    api
      .history(selectedId)
      .then((h) => setHistory(h.map((e) => e.command)))
      .catch(() => setHistory([]));

    // Switching servers resets the tab choice -- the previous pick was about
    // a different discovery report and may not even apply to this one.
    tabTouched.current = false;
    setTab("console");
    refreshDiscovery(selectedId);
  }, [selectedId, refreshDiscovery]);

  // Once the report for the current server resolves, land on the highest
  // rung it supports -- unless the user has already picked a tab by hand
  // since the last server switch.
  useEffect(() => {
    if (!tabTouched.current) setTab(defaultTab(report));
  }, [report]);

  const chooseTab = (t: Tab) => {
    tabTouched.current = true;
    setTab(t);
  };

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

  const runPrefill = (command: string) => {
    chooseTab("console");
    setPrefill({ text: command, nonce: Date.now() });
  };

  const rescan = async () => {
    if (!selectedId) return;
    setRescanning(true);
    try {
      const fresh = await api.discover(selectedId);
      setReport(fresh);
      setReportError(null);
    } catch (e) {
      setReportError(e instanceof Error ? e.message : String(e));
    } finally {
      setRescanning(false);
    }
  };

  const commands = useMemo<Command[]>(() => {
    const list: Command[] = [
      { id: "add", label: "Add server…", run: () => setDialog({ open: true, server: null }) },
      { id: "theme", label: "Toggle light / dark", run: () => document.documentElement.classList.toggle("dark") },
      { id: "open-templates", label: "Browse templates", run: () => setView("templates") },
    ];

    if (selected) {
      list.push(
        { id: "edit", label: `Edit “${selected.name}”…`, run: () => setDialog({ open: true, server: selected }) },
        { id: "connect", label: `Reconnect “${selected.name}”`, run: () => api.connect(selected.id).then(refreshServers) },
        { id: "disconnect", label: `Disconnect “${selected.name}”`, run: () => api.disconnect(selected.id).then(refreshServers) },
        { id: "rescan", label: `Re-scan “${selected.name}”`, run: rescan },
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
      if (report?.capabilities.options) {
        list.push({ id: "open-settings", label: "Open settings", run: () => chooseTab("settings") });
      }
      if (report?.capabilities.commands) {
        list.push({ id: "open-commands", label: "Open commands", run: () => chooseTab("commands") });
      }
      for (const m of macros) {
        list.push({ id: `macro-${m.id}`, label: m.name, hint: m.command, run: () => execute(m.command) });
      }
      for (const cmd of history.slice(0, 20)) {
        list.push({ id: `history-${cmd}`, label: cmd, hint: "recent", run: () => execute(cmd) });
      }
    }
    return list;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected, macros, history, refreshServers, report]);

  return (
    <div className="flex h-full">
      <Sidebar
        servers={servers}
        selectedId={selectedId}
        onSelect={(id) => {
          setView("server");
          setSelectedId(id);
        }}
        onAdd={() => setDialog({ open: true, server: null })}
        onOpenPalette={() => setPaletteOpen(true)}
        templatesActive={view === "templates"}
        onOpenTemplates={() => setView("templates")}
      />

      <main className="flex min-w-0 flex-1 flex-col">
        {view === "templates" ? (
          <TemplatesPanel />
        ) : loadError ? (
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
            <header className="flex items-center gap-3.5 border-b px-4.5 py-3">
              <div className="min-w-0">
                <h2 className="truncate text-[15px] font-semibold">{selected.name}</h2>
                <p className="truncate font-mono text-xs" style={{ color: "var(--color-text-muted)" }}>
                  {selected.addr} · {selected.protocol}
                </p>
              </div>

              <div className="ml-auto flex items-center gap-2">
                <StatusBadge status={selected.status} />
                {selected.status === "connected" ? (
                  <button onClick={() => api.disconnect(selected.id).then(refreshServers)} className="btn btn-ghost text-xs">
                    disconnect
                  </button>
                ) : (
                  <button onClick={() => api.connect(selected.id).then(refreshServers)} className="btn btn-ghost text-xs">
                    connect
                  </button>
                )}
                <button onClick={() => setDialog({ open: true, server: selected })} className="btn btn-ghost text-xs">
                  edit
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

            <DiscoveryStrip
              report={report}
              error={reportError}
              server={selected}
              rescanning={rescanning}
              onRescan={rescan}
            />

            <div className="flex gap-0.5 border-b px-4.5 pt-2.5" role="tablist" style={{ borderColor: "var(--color-border-soft)" }}>
              {(report?.capabilities.options ?? true) && (
                <TabButton label="Settings" count={report?.options.length} active={tab === "settings"} onClick={() => chooseTab("settings")} />
              )}
              {(report?.capabilities.commands ?? true) && (
                <TabButton label="Commands" count={report?.commands.length} active={tab === "commands"} onClick={() => chooseTab("commands")} />
              )}
              <TabButton label="Console" active={tab === "console"} onClick={() => chooseTab("console")} />
            </div>

            <div className="min-h-0 flex-1 overflow-y-auto">
              {tab === "settings" && (
                <SettingsPanel
                  serverId={selected.id}
                  connected={selected.status === "connected"}
                  report={report}
                  onReport={setReport}
                />
              )}
              {tab === "commands" && (
                <CommandsPanel
                  serverId={selected.id}
                  connected={selected.status === "connected"}
                  report={report}
                  onExecute={execute}
                  onRunPrefill={runPrefill}
                />
              )}
              {tab === "console" && (
                <div className="flex min-h-full flex-col">
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
                    commandNames={report?.commands.map((c) => c.name) ?? []}
                    prefill={prefill}
                    onSubmit={execute}
                  />
                </div>
              )}
            </div>
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
    connected: "connected",
    connecting: "connecting…",
    auth_failed: "auth failed",
    disconnected: "disconnected",
  }[status];

  const colour = {
    connected: "var(--color-ok)",
    connecting: "var(--color-accent)",
    auth_failed: "var(--color-danger)",
    disconnected: "var(--color-text-muted)",
  }[status];

  return (
    <span className="flex items-center gap-1.5 font-mono text-[11px] font-medium" style={{ color: colour }}>
      <span className="size-2 rounded-full" style={{ background: colour }} />
      {label}
    </span>
  );
}

function TabButton({
  label,
  count,
  active,
  onClick,
}: {
  label: string;
  count?: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className="-mb-px cursor-pointer border-b-2 px-3.5 pt-2 pb-2.5 text-[12.5px] font-medium"
      style={{
        color: active ? "var(--color-text)" : "var(--color-text-muted)",
        borderColor: active ? "var(--color-accent)" : "transparent",
      }}
    >
      {label}
      {count !== undefined && (
        <span className="ml-1.5 font-mono text-[10.5px]" style={{ color: "var(--color-text-faint)" }}>
          {count}
        </span>
      )}
    </button>
  );
}

/** The discovery strip: what the server turned out to be, and how much of it
 * the daemon could interrogate. Every fact here traces back to the report --
 * nothing is inferred from a game list, because there isn't one. */
function DiscoveryStrip({
  report,
  error,
  server,
  rescanning,
  onRescan,
}: {
  report: DiscoveryReport | null;
  error: string | null;
  server: Server;
  rescanning: boolean;
  onRescan: () => void;
}) {
  return (
    <div
      className="flex flex-wrap items-center gap-2 border-b px-4.5 py-2.5"
      style={{ borderColor: "var(--color-border-soft)", background: "var(--color-surface-raised)" }}
    >
      <span
        className="font-mono text-[10.5px] font-semibold tracking-[.1em] uppercase"
        style={{ color: "var(--color-text-faint)" }}
      >
        discovered
      </span>

      {report ? (
        <>
          <span className="chip chip-accent">{report.label}</span>
          {report.capabilities.options && (
            <span className="chip">
              <b style={{ color: "var(--color-text)", fontWeight: 600 }}>{report.options.length}</b> options
            </span>
          )}
          {report.capabilities.commands && (
            <span className="chip">
              <b style={{ color: "var(--color-text)", fontWeight: 600 }}>{report.commands.length}</b> commands
            </span>
          )}
          <span className="chip">
            options{" "}
            <span style={{ color: report.capabilities.options ? "var(--color-ok)" : "var(--color-text-faint)" }}>
              {report.capabilities.options ? "✓" : "✗"}
            </span>
          </span>
          <span className="chip">
            write{" "}
            <span style={{ color: report.capabilities.optionWrite ? "var(--color-ok)" : "var(--color-text-faint)" }}>
              {report.capabilities.optionWrite ? "✓" : "✗"}
            </span>
          </span>
        </>
      ) : (
        <span className="chip">not discovered yet</span>
      )}

      {/* Surfaced independently of whether a (possibly stale) report exists --
          a re-scan failure should never be hidden behind an old success. */}
      {error && (
        <span className="chip" style={{ color: "var(--color-danger)", borderColor: "var(--color-danger)" }}>
          {error}
        </span>
      )}

      <button
        onClick={onRescan}
        disabled={rescanning || server.status !== "connected"}
        className="ml-auto cursor-pointer font-mono text-[11px] disabled:cursor-not-allowed disabled:opacity-40"
        style={{ color: "var(--color-text-faint)" }}
        onMouseEnter={(e) => {
          if (!e.currentTarget.disabled) e.currentTarget.style.color = "var(--color-accent)";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.color = "var(--color-text-faint)";
        }}
        title={server.status !== "connected" ? "Connect first to re-scan" : "Re-run discovery"}
      >
        {rescanning ? "scanning…" : "↻ re-scan"}
      </button>
    </div>
  );
}
