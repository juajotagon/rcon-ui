import { useMemo, useState } from "react";
import { games, type CatalogOption } from "../catalog";
import { OptionRow, type Row } from "./SettingsPanel";
import { CommandCells } from "./CommandsPanel";

/** Groups a game's curated options by `section`, "Other" trailing -- the
 * same ordering rule SettingsPanel applies to a live report, kept here as a
 * small standalone copy rather than a shared export because the input shape
 * (CatalogOption[] vs. a live Row[]) differs enough that sharing the memo
 * itself would need its own abstraction for one call site each.
 */
function groupBySection(options: CatalogOption[]): [string, CatalogOption[]][] {
  const bySection = new Map<string, CatalogOption[]>();
  for (const opt of options) {
    const name = opt.section ?? "Other";
    if (!bySection.has(name)) bySection.set(name, []);
    bySection.get(name)!.push(opt);
  }
  const entries = [...bySection.entries()];
  entries.sort(([a], [b]) => {
    if (a === "Other") return 1;
    if (b === "Other") return -1;
    return 0;
  });
  return entries;
}

/** TemplatesPanel is a static, connection-independent catalog: what a game's
 * RCON surface looks like, curated by hand and baked into the app. Nothing
 * here reads from a server -- every field traces back to catalog/*.ts, which
 * is also what live enrichment (enrich/index.ts) draws from, so the options
 * shown for Zomboid/Minecraft here are provably the same ones Settings uses.
 */
export function TemplatesPanel() {
  const [selectedId, setSelectedId] = useState(games[0].id);
  const [filter, setFilter] = useState("");
  const game = games.find((g) => g.id === selectedId) ?? games[0];

  const sections = useMemo(() => groupBySection(game.options), [game]);

  const needle = filter.trim().toLowerCase();
  const visibleCommands = useMemo(
    () =>
      game.commands.filter(
        (c) =>
          !needle ||
          c.name.toLowerCase().includes(needle) ||
          c.description.toLowerCase().includes(needle),
      ),
    [game, needle],
  );

  return (
    <div className="flex min-h-full">
      <nav
        aria-label="Games"
        className="w-56 shrink-0 overflow-y-auto border-r py-2"
        style={{ borderColor: "var(--color-border-soft)", background: "var(--color-surface-sunken)" }}
      >
        {games.map((g) => {
          const active = g.id === selectedId;
          return (
            <button
              key={g.id}
              onClick={() => setSelectedId(g.id)}
              aria-current={active}
              className="block w-full border-l-2 px-3.5 py-2 text-left"
              style={{
                borderLeftColor: active ? "var(--color-accent)" : "transparent",
                background: active ? "var(--color-surface)" : "transparent",
                color: active ? "var(--color-text)" : "var(--color-text-muted)",
              }}
            >
              <span className="block truncate text-[13px] font-medium">{g.name}</span>
              <span className="mt-0.5 block font-mono text-[10.5px]" style={{ color: "var(--color-text-faint)" }}>
                {g.capabilities.options ? `${g.options.length} options` : "no options"} ·{" "}
                {g.commands.length} commands
              </span>
            </button>
          );
        })}
      </nav>

      <div className="min-w-0 flex-1 overflow-y-auto">
        <div
          className="flex flex-wrap items-center gap-2 border-b px-4.5 py-2.5"
          style={{ borderColor: "var(--color-border-soft)", background: "var(--color-surface-raised)" }}
        >
          <span className="chip chip-accent">{game.name}</span>
          {game.defaultPort !== undefined && <span className="chip">default port {game.defaultPort}</span>}
          <span className="chip">
            options{" "}
            <span style={{ color: game.capabilities.options ? "var(--color-ok)" : "var(--color-text-faint)" }}>
              {game.capabilities.options ? "✓" : "✗"}
            </span>
          </span>
          <span className="chip">
            write{" "}
            <span style={{ color: game.capabilities.optionWrite ? "var(--color-ok)" : "var(--color-text-faint)" }}>
              {game.capabilities.optionWrite ? "✓" : "✗"}
            </span>
          </span>
          <span className="chip">
            commands{" "}
            <span style={{ color: game.capabilities.commands ? "var(--color-ok)" : "var(--color-text-faint)" }}>
              {game.capabilities.commands ? "✓" : "✗"}
            </span>
          </span>
        </div>

        {game.caveats.length > 0 && (
          <div className="px-4.5 pt-3 pb-1">
            <h3
              className="mb-1.5 font-mono text-[10.5px] font-semibold tracking-[.13em] uppercase"
              style={{ color: "var(--color-text-faint)" }}
            >
              Caveats
            </h3>
            <ul className="space-y-1">
              {game.caveats.map((c, i) => (
                <li
                  key={i}
                  className="flex gap-2 rounded-md border px-2.5 py-1.5 text-[12px] leading-snug"
                  style={{ borderColor: "var(--color-border-soft)", color: "var(--color-text-muted)" }}
                >
                  <span aria-hidden style={{ color: "var(--color-warn)" }}>
                    ⚠
                  </span>
                  <span>{c}</span>
                </li>
              ))}
            </ul>
          </div>
        )}

        <div className="px-4.5 pt-2 pb-0.5">
          <h3
            className="mt-2 mb-1 flex items-baseline gap-2 font-mono text-[10.5px] font-semibold tracking-[.13em] uppercase"
            style={{ color: "var(--color-text-faint)" }}
          >
            Options
          </h3>
          {!game.capabilities.options ? (
            <p className="pb-3 text-[12.5px]" style={{ color: "var(--color-text-muted)" }}>
              No settings dump over RCON — configure via console/cvars/ini.
            </p>
          ) : (
            sections.map(([name, opts]) => (
              <div key={name}>
                <h4
                  className="mt-3.5 mb-1 flex items-baseline gap-2 font-mono text-[10.5px] font-semibold tracking-[.13em] uppercase"
                  style={{ color: "var(--color-text-faint)" }}
                >
                  {name} <span style={{ color: "var(--color-accent)" }}>{opts.length}</span>
                </h4>
                <div>
                  {opts.map((opt) => {
                    const row: Row = { key: opt.key, value: opt.example, type: "string", meta: opt };
                    return (
                      <OptionRow
                        key={opt.key}
                        row={row}
                        staged={undefined}
                        error={undefined}
                        readOnly
                        onChange={() => {}}
                      />
                    );
                  })}
                </div>
              </div>
            ))
          )}
        </div>

        <div className="px-4.5 pt-3 pb-6">
          <div className="mb-1.5 flex items-center gap-2.5">
            <h3
              className="font-mono text-[10.5px] font-semibold tracking-[.13em] uppercase"
              style={{ color: "var(--color-text-faint)" }}
            >
              Commands <span style={{ color: "var(--color-accent)" }}>{game.commands.length}</span>
            </h3>
            <label
              className="ml-auto flex max-w-70 flex-1 items-center gap-2 rounded-md border px-2.5 py-1"
              style={{ background: "var(--color-surface-sunken)" }}
            >
              <span className="font-mono text-xs" style={{ color: "var(--color-text-faint)" }}>
                /
              </span>
              <input
                type="search"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                placeholder={`filter ${game.commands.length} commands…`}
                aria-label="Filter commands"
                className="flex-1 bg-transparent font-mono text-[12px] outline-none"
              />
            </label>
          </div>
          {game.commands.length === 0 ? (
            <p className="text-[12.5px]" style={{ color: "var(--color-text-muted)" }}>
              No commands documented for this game yet.
            </p>
          ) : (
            visibleCommands.map((cmd) => (
              <div
                key={cmd.name}
                className="grid grid-cols-[200px_minmax(0,1fr)] items-center gap-4 rounded-md border-t px-2.5 py-2 first:border-t-0"
                style={{ borderColor: "var(--color-border-soft)" }}
              >
                <CommandCells name={cmd.name} description={cmd.description} usage={cmd.usage} />
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
