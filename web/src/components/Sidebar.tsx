import { useMemo } from "react";
import type { Server, Status } from "../api";

const STATUS_COLOUR: Record<Status, string> = {
  connected: "var(--color-ok)",
  connecting: "var(--color-warn)",
  auth_failed: "var(--color-danger)",
  disconnected: "var(--color-text-muted)",
};

const STATUS_LABEL: Record<Status, string> = {
  connected: "Connected",
  connecting: "Connecting…",
  auth_failed: "Wrong password",
  disconnected: "Disconnected",
};

/** Sidebar lists servers, optionally grouped.
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
    <aside
      className="flex w-60 shrink-0 flex-col border-r"
      style={{ background: "var(--color-surface-raised)" }}
    >
      <header className="flex items-center gap-2 border-b px-3 py-2.5">
        <span className="text-sm font-semibold tracking-tight">rcon-ui</span>
        <button
          onClick={onOpenPalette}
          className="btn btn-ghost ml-auto px-1.5 py-1 text-xs"
          title="Command palette"
        >
          <kbd className="rounded border px-1 py-0.5 font-mono text-[10px]">⌘K</kbd>
        </button>
      </header>

      <nav className="min-h-0 flex-1 overflow-y-auto py-1">
        {servers.length === 0 && (
          <p className="px-3 py-6 text-center text-xs" style={{ color: "var(--color-text-muted)" }}>
            No servers yet.
          </p>
        )}

        {groups.map(([group, items]) => (
          <div key={group} className="mb-1">
            {group && (
              <h2
                className="px-3 pt-2 pb-1 text-[10px] font-semibold tracking-wider uppercase"
                style={{ color: "var(--color-text-muted)" }}
              >
                {group}
              </h2>
            )}
            {items.map((s) => (
              <button
                key={s.id}
                onClick={() => onSelect(s.id)}
                aria-current={s.id === selectedId}
                className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm transition-colors ${
                  s.id === selectedId ? "font-medium" : ""
                }`}
                style={
                  s.id === selectedId
                    ? { background: "var(--color-surface-sunken)" }
                    : undefined
                }
              >
                <span
                  className="size-2 shrink-0 rounded-full"
                  style={{ background: STATUS_COLOUR[s.status] }}
                  title={STATUS_LABEL[s.status]}
                  aria-label={STATUS_LABEL[s.status]}
                />
                <span className="truncate">{s.name}</span>
              </button>
            ))}
          </div>
        ))}
      </nav>

      <footer className="border-t p-2">
        <button onClick={onAdd} className="btn btn-ghost w-full justify-start text-sm">
          <span className="text-base leading-none">+</span> Add server
        </button>
      </footer>
    </aside>
  );
}
