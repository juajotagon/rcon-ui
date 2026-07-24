import { useEffect, useMemo, useRef, useState } from "react";

export interface Command {
  id: string;
  label: string;
  hint?: string;
  run: () => void;
}

/** CommandPalette is the keyboard route to every action.
 *
 * Every entry here is also reachable as a visible control elsewhere. The
 * palette is an accelerator for people who already know what they want, never
 * the only way to reach something -- discoverability by pointer and speed by
 * keyboard are both requirements, not alternatives.
 */
export function CommandPalette({
  commands,
  onClose,
}: {
  commands: Command[];
  onClose: () => void;
}) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLUListElement>(null);

  const matches = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return commands;
    return commands.filter(
      (c) =>
        c.label.toLowerCase().includes(needle) || c.hint?.toLowerCase().includes(needle),
    );
  }, [commands, query]);

  useEffect(() => setActive(0), [query]);

  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
        return;
      }
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setActive((i) => Math.min(i + 1, matches.length - 1));
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setActive((i) => Math.max(i - 1, 0));
        return;
      }
      if (e.key === "Enter") {
        e.preventDefault();
        const chosen = matches[active];
        if (chosen) {
          chosen.run();
          onClose();
        }
      }
    };

    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [matches, active, onClose]);

  // Keep the highlighted row in view when navigating by keyboard.
  useEffect(() => {
    listRef.current?.children[active]?.scrollIntoView({ block: "nearest" });
  }, [active]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/50 p-4 pt-[15vh]"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-lg overflow-hidden rounded-lg border shadow-xl"
        style={{ background: "var(--color-surface-raised)" }}
      >
        <input
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Type a command…"
          className="w-full border-b bg-transparent px-4 py-3 text-sm outline-none"
        />

        <ul ref={listRef} className="max-h-80 overflow-y-auto py-1">
          {matches.length === 0 && (
            <li className="px-4 py-6 text-center text-xs" style={{ color: "var(--color-text-muted)" }}>
              No matching commands
            </li>
          )}
          {matches.map((c, i) => (
            <li key={c.id}>
              <button
                onMouseEnter={() => setActive(i)}
                onClick={() => {
                  c.run();
                  onClose();
                }}
                className="flex w-full items-center gap-3 px-4 py-2 text-left text-sm"
                style={i === active ? { background: "var(--color-surface-sunken)" } : undefined}
              >
                <span className="truncate">{c.label}</span>
                {c.hint && (
                  <span
                    className="ml-auto truncate font-mono text-xs"
                    style={{ color: "var(--color-text-muted)" }}
                  >
                    {c.hint}
                  </span>
                )}
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
