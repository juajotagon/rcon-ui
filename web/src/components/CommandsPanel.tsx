import { useEffect, useMemo, useState } from "react";
import type { DiscoveryReport } from "../api";

/** CommandsPanel is the catalog view: everything the daemon parsed out of the
 * server's own help reply, filterable and pinnable. Nothing here is run
 * except through `onExecute`, which is the same execute call the console
 * uses -- a command run from this panel echoes in the console exactly like
 * one typed by hand.
 */
export function CommandsPanel({
  serverId,
  connected,
  report,
  onExecute,
  onRunPrefill,
}: {
  serverId: string;
  connected: boolean;
  report: DiscoveryReport | null;
  onExecute: (command: string) => void;
  onRunPrefill: (command: string) => void;
}) {
  const pinsKey = `rcon-ui:pins:${serverId}`;
  const [pins, setPins] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState("");

  useEffect(() => {
    try {
      const raw = localStorage.getItem(pinsKey);
      setPins(new Set(raw ? (JSON.parse(raw) as string[]) : []));
    } catch {
      setPins(new Set());
    }
  }, [pinsKey]);

  const togglePin = (name: string) => {
    setPins((prev) => {
      const next = new Set(prev);
      next.has(name) ? next.delete(name) : next.add(name);
      localStorage.setItem(pinsKey, JSON.stringify([...next]));
      return next;
    });
  };

  const rows = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    const all = (report?.commands ?? []).filter(
      (c) => !needle || c.name.toLowerCase().includes(needle) || c.description.toLowerCase().includes(needle),
    );
    // Stable partition: pinned first, original catalog order preserved within
    // each half, so pinning never reshuffles the rest of the list.
    const pinned = all.filter((c) => pins.has(c.name));
    const rest = all.filter((c) => !pins.has(c.name));
    return [...pinned, ...rest];
  }, [report, filter, pins]);

  if (!report) {
    return <Empty text="No discovery yet — use ↻ re-scan above." />;
  }
  if (!report.capabilities.commands) {
    return <Empty text="This server did not report a command catalog." />;
  }

  return (
    <div className="flex min-h-full flex-col">
      <div
        className="sticky top-0 z-10 flex items-center gap-2.5 px-4.5 pt-3.5 pb-1"
        style={{ background: "var(--color-surface)" }}
      >
        <label
          className="flex max-w-85 flex-1 items-center gap-2 rounded-md border px-2.5 py-1.5"
          style={{ background: "var(--color-surface-sunken)" }}
        >
          <span className="font-mono text-xs" style={{ color: "var(--color-text-faint)" }}>
            /
          </span>
          <input
            type="search"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder={`filter ${report.commands.length} commands…`}
            aria-label="Filter commands"
            className="flex-1 bg-transparent font-mono text-[12.5px] outline-none"
          />
        </label>
        <span className="text-[11.5px]" style={{ color: "var(--color-text-faint)" }}>
          catalog parsed from the server's own command help · ★ pinned
        </span>
      </div>

      <div className="px-4.5 pt-1 pb-4">
        {rows.map((cmd) => {
          const pinned = pins.has(cmd.name);
          const immediate = cmd.usage.trim() === "";
          return (
            <div
              key={cmd.name}
              className="grid grid-cols-[200px_minmax(0,1fr)_auto] items-center gap-4 rounded-md border-t px-2.5 py-2 first:border-t-0"
              style={{ borderColor: "var(--color-border-soft)" }}
            >
              <span className="font-mono text-[12.5px] font-medium">
                <button
                  onClick={() => togglePin(cmd.name)}
                  aria-pressed={pinned}
                  aria-label={pinned ? `Unpin ${cmd.name}` : `Pin ${cmd.name}`}
                  className="mr-1.5 cursor-pointer text-[11px]"
                  style={{ color: pinned ? "var(--color-accent)" : "var(--color-text-faint)" }}
                >
                  ★
                </button>
                {cmd.name}
              </span>
              <span className="text-xs" style={{ color: "var(--color-text-muted)" }}>
                {cmd.description}
                {cmd.usage && (
                  <span className="ml-1.5 font-mono" style={{ color: "var(--color-text-faint)" }}>
                    Use: {cmd.usage}
                  </span>
                )}
              </span>
              <button
                disabled={!connected}
                onClick={() => (immediate ? onExecute(cmd.name) : onRunPrefill(cmd.name + " "))}
                className="cursor-pointer rounded border px-2.5 py-1 font-mono text-[11px] disabled:cursor-not-allowed disabled:opacity-40"
                style={{ color: "var(--color-text-faint)", borderColor: "var(--color-border)" }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = "var(--color-accent)";
                  e.currentTarget.style.borderColor = "var(--color-accent)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = "var(--color-text-faint)";
                  e.currentTarget.style.borderColor = "var(--color-border)";
                }}
              >
                {immediate ? "run" : "run…"}
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function Empty({ text }: { text: string }) {
  return (
    <div className="flex flex-1 items-center justify-center p-10 text-center">
      <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
        {text}
      </p>
    </div>
  );
}
