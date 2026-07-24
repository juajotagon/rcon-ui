import { useEffect, useMemo, useRef, useState } from "react";
import type { RconEvent } from "../api";

/** Console renders the merged event stream.
 *
 * Built as a filterable list of independent, source-tagged events rather than
 * a list of command/response pairs. In v1 there is exactly one source, so the
 * filter bar is nearly redundant -- but the pairing shape cannot represent a
 * log line, which has no command half, and adopting it would mean rewriting
 * this component when log sources land.
 */
export function Console({ events, emptyHint }: { events: RconEvent[]; emptyHint: string }) {
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);
  const pinnedToBottom = useRef(true);

  const sources = useMemo(() => [...new Set(events.map((e) => e.source))].sort(), [events]);

  const visible = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    return events.filter(
      (e) => !hidden.has(e.source) && (!needle || e.line.toLowerCase().includes(needle)),
    );
  }, [events, hidden, filter]);

  // Follow new output, but stop the moment the user scrolls up to read
  // something -- yanking them back to the bottom mid-read is the single most
  // irritating thing a console can do.
  useEffect(() => {
    if (pinnedToBottom.current && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [visible.length]);

  const onScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    pinnedToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  };

  const toggle = (source: string) =>
    setHidden((prev) => {
      const next = new Set(prev);
      next.has(source) ? next.delete(source) : next.add(source);
      return next;
    });

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {(sources.length > 1 || filter) && (
        <div className="flex items-center gap-2 border-b px-3 py-1.5 text-xs">
          {sources.map((s) => (
            <button
              key={s}
              onClick={() => toggle(s)}
              className={`rounded-full border px-2 py-0.5 transition-opacity ${
                hidden.has(s) ? "opacity-40" : ""
              }`}
            >
              {s}
            </button>
          ))}
          <input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter…"
            className="ml-auto w-48 rounded border bg-transparent px-2 py-0.5 outline-none"
          />
        </div>
      )}

      <div
        ref={scrollRef}
        onScroll={onScroll}
        className="min-h-0 flex-1 overflow-y-auto px-3 py-2 font-mono text-[13px] leading-relaxed"
        style={{ background: "var(--color-surface-sunken)" }}
      >
        {visible.length === 0 ? (
          // An honest empty state. RCON cannot stream server logs, so there is
          // nothing arriving to wait for -- showing a spinner here would
          // promise output that will never come.
          <p className="px-1 py-8 text-center text-xs" style={{ color: "var(--color-text-muted)" }}>
            {emptyHint}
          </p>
        ) : (
          visible.map((e) => <Line key={e.seq} event={e} />)
        )}
      </div>
    </div>
  );
}

function Line({ event }: { event: RconEvent }) {
  const time = new Date(event.at).toLocaleTimeString(undefined, { hour12: false });

  if (event.stream === "command") {
    return (
      <div className="flex gap-2 pt-2">
        <span className="shrink-0 opacity-40">{time}</span>
        <span style={{ color: "var(--color-accent)" }} className="shrink-0 font-semibold">
          ›
        </span>
        <span className="font-semibold break-all">{event.line}</span>
      </div>
    );
  }

  if (event.stream === "status") {
    return (
      <div className="flex gap-2 text-xs opacity-70">
        <span className="shrink-0 opacity-60">{time}</span>
        <span className="italic break-all">{event.line}</span>
      </div>
    );
  }

  if (event.stream === "error") {
    return (
      <div className="flex gap-2" style={{ color: "var(--color-danger)" }}>
        <span className="shrink-0 opacity-60">{time}</span>
        <span className="break-all whitespace-pre-wrap">{event.line}</span>
      </div>
    );
  }

  // A response. Empty is meaningful -- plenty of commands succeed silently --
  // so say so rather than rendering a blank line the user cannot interpret.
  return (
    <div className="flex gap-2">
      <span className="shrink-0 opacity-40">{time}</span>
      {event.line === "" ? (
        <span className="text-xs italic opacity-50">(no output)</span>
      ) : (
        <span className="break-all whitespace-pre-wrap">{event.line}</span>
      )}
    </div>
  );
}
