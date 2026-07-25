import { useMemo } from "react";
import type { Server, Status } from "../api";

const DOT_COLOUR: Record<Status, string> = {
  connected: "var(--color-ok)",
  connecting: "var(--color-accent)", // pulses -- "busy", not yet a verdict either way
  auth_failed: "var(--color-danger)",
  disconnected: "var(--color-text-faint)",
};

const STATUS_LABEL: Record<Status, string> = {
  connected: "Connected",
  connecting: "Connecting…",
  auth_failed: "Wrong password",
  disconnected: "Disconnected",
};

/** Turns a persisted fingerprint slug into rail-chip text.
 *
 * This is string formatting, not a game table -- it has no opinion about
 * which fingerprints exist, so a fingerprint this build has never heard of
 * still renders sensibly instead of needing a matching entry somewhere.
 */
function fingerprintLabel(server: Server): string {
  if (server.status === "connecting" && !server.game) return "scanning…";
  if (!server.game || server.game === "unknown") return "unidentified";
  return server.game
    .split(/[-_\s]+/)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}

/** Sidebar is the server rail: a status-ring tile per server, grouped, with a
 * fingerprint chip driven entirely by `server.game` (the daemon's persisted
 * discovery result) -- never a hardcoded game list. An empty or "unknown"
 * fingerprint gets the same "unidentified" treatment a brand-new protocol
 * would, which is the point: nothing here assumes what a server turns out to
 * be.
 *
 * A flat list with user-defined groups rather than a nested resource tree:
 * tree explorers suit Kubernetes, where resources genuinely nest, but a set of
 * game servers does not, and imposing a hierarchy would add clicks for nothing.
 */
export function Sidebar({
  servers,
  selectedId,
  onSelect,
  onAdd,
  onOpenPalette,
}: {
  servers: Server[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onAdd: () => void;
  onOpenPalette: () => void;
}) {
  const groups = useMemo(() => {
    const byGroup = new Map<string, Server[]>();
    for (const s of servers) {
      const key = s.group || "";
      if (!byGroup.has(key)) byGroup.set(key, []);
      byGroup.get(key)!.push(s);
    }
    return [...byGroup.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [servers]);

  return (
    <nav
      aria-label="Servers"
      className="flex w-60 shrink-0 flex-col"
      style={{ background: "var(--color-surface-sunken)" }}
    >
      <header className="flex items-center gap-2 px-3.5 py-3">
        <span className="font-mono text-[13px] font-bold tracking-wide">
          rcon<em className="not-italic" style={{ color: "var(--color-accent)" }}>·</em>ui
        </span>
        <button
          onClick={onOpenPalette}
          className="btn btn-ghost ml-auto px-1.5 py-1 text-xs"
          title="Command palette"
        >
          <kbd
            className="rounded border px-1 py-0.5 font-mono text-[10px]"
            style={{ color: "var(--color-text-faint)" }}
          >
            ⌘K
          </kbd>
        </button>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto pb-1">
        {servers.length === 0 && (
          <p className="px-3 py-6 text-center text-xs" style={{ color: "var(--color-text-muted)" }}>
            No servers yet.
          </p>
        )}

        {groups.map(([group, items]) => (
          <div key={group}>
            {group && (
              <h2
                className="px-4 pt-3 pb-1 font-mono text-[10px] font-semibold tracking-[.14em] uppercase"
                style={{ color: "var(--color-text-faint)" }}
              >
                {group}
              </h2>
            )}
            {items.map((s) => {
              const selected = s.id === selectedId;
              return (
                <button
                  key={s.id}
                  onClick={() => onSelect(s.id)}
                  aria-current={selected}
                  className="block w-full border-l-2 py-2.5 pr-3.5 pl-3 text-left"
                  style={{
                    borderLeftColor: selected ? "var(--color-accent)" : "transparent",
                    background: selected ? "var(--color-surface)" : "transparent",
                  }}
                  onMouseEnter={(e) => {
                    if (!selected) e.currentTarget.style.background = "var(--color-surface)";
                  }}
                  onMouseLeave={(e) => {
                    if (!selected) e.currentTarget.style.background = "transparent";
                  }}
                >
                  <span className="flex items-center gap-2 text-[13px] font-semibold">
                    <span
                      className="size-[7px] shrink-0 rounded-full"
                      style={{
                        background: DOT_COLOUR[s.status],
                        boxShadow:
                          s.status === "connected"
                            ? "0 0 0 3px color-mix(in srgb, var(--color-ok) 22%, transparent)"
                            : undefined,
                        animation: s.status === "connecting" ? "pulse 1.2s ease-in-out infinite" : undefined,
                      }}
                      title={STATUS_LABEL[s.status]}
                      aria-label={STATUS_LABEL[s.status]}
                    />
                    <span className="truncate">{s.name}</span>
                  </span>
                  <span
                    className="mt-0.5 block truncate font-mono text-[11px]"
                    style={{ color: "var(--color-text-muted)" }}
                  >
                    {s.addr}
                  </span>
                  <span
                    className="chip mt-[3px] inline-block px-[5px] py-px text-[10px]"
                    style={{
                      color: selected ? "var(--color-accent)" : "var(--color-text-faint)",
                      borderColor: selected ? "var(--color-accent)" : "var(--color-border)",
                    }}
                  >
                    {fingerprintLabel(s)}
                  </span>
                </button>
              );
            })}
          </div>
        ))}
      </div>

      <footer className="p-3">
        <button
          onClick={onAdd}
          className="w-full cursor-pointer rounded-md border border-dashed px-2 py-2 font-mono text-xs"
          style={{ color: "var(--color-text-muted)", borderColor: "var(--color-border)" }}
          onMouseEnter={(e) => {
            e.currentTarget.style.color = "var(--color-accent)";
            e.currentTarget.style.borderColor = "var(--color-accent)";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.color = "var(--color-text-muted)";
            e.currentTarget.style.borderColor = "var(--color-border)";
          }}
        >
          + add server
        </button>
      </footer>
    </nav>
  );
}
