import { useMemo, useState } from "react";
import { api, type DiscoveredOption, type DiscoveryReport } from "../api";
import { metaFor, type OptionMeta } from "../enrich";

/** Builds the exact command that will be sent for one staged edit.
 *
 * `writeTemplate` is the daemon's, not ours -- substituting it here (rather
 * than assembling a command string some other way) is what makes the staging
 * bar's preview and the command actually sent provably the same string.
 */
function buildCommand(template: string, key: string, value: string): string {
  return template.replaceAll("{key}", key).replaceAll("{value}", value);
}

// Exported so TemplatesPanel can render curated catalog entries through the
// exact same row/control visuals -- both are stateless functions of `Row`,
// so reuse is a straight export rather than a fork or a second component.
export interface Row {
  key: string;
  value: string;
  type: DiscoveredOption["type"];
  meta?: OptionMeta;
}

export function SettingsPanel({
  serverId,
  connected,
  report,
  onReport,
}: {
  serverId: string;
  connected: boolean;
  report: DiscoveryReport | null;
  onReport: (report: DiscoveryReport) => void;
}) {
  const [edits, setEdits] = useState<Record<string, string>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [filter, setFilter] = useState("");
  const [applying, setApplying] = useState(false);

  const rows = useMemo<Row[]>(
    () =>
      (report?.options ?? []).map((o) => ({
        ...o,
        meta: report ? metaFor(report.fingerprint, o.key) : undefined,
      })),
    [report],
  );

  const sections = useMemo(() => {
    const bySection = new Map<string, Row[]>();
    for (const row of rows) {
      const name = row.meta?.section ?? "Other · undocumented";
      if (!bySection.has(name)) bySection.set(name, []);
      bySection.get(name)!.push(row);
    }
    // Curated sections keep the daemon's option order; "Other" always trails
    // so unrecognised keys read as a fallback, not as just another category.
    const entries = [...bySection.entries()];
    entries.sort(([a], [b]) => {
      if (a.startsWith("Other")) return 1;
      if (b.startsWith("Other")) return -1;
      return 0;
    });
    return entries;
  }, [rows]);

  const needle = filter.trim().toLowerCase();
  const matches = (row: Row) =>
    !needle ||
    row.key.toLowerCase().includes(needle) ||
    (row.meta?.label ?? "").toLowerCase().includes(needle) ||
    (row.meta?.description ?? "").toLowerCase().includes(needle) ||
    row.value.toLowerCase().includes(needle);

  const dirtyKeys = useMemo(
    () => Object.keys(edits).filter((k) => rows.some((r) => r.key === k && edits[k] !== r.value)),
    [edits, rows],
  );

  const setValue = (key: string, value: string) =>
    setEdits((prev) => ({ ...prev, [key]: value }));

  const discard = () => {
    setEdits({});
    setErrors({});
  };

  const apply = async () => {
    if (!report) return;
    setApplying(true);
    const nextErrors: Record<string, string> = {};
    const applied: string[] = [];
    for (const key of dirtyKeys) {
      const cmd = buildCommand(report.writeTemplate, key, edits[key]);
      try {
        await api.execute(serverId, cmd);
        applied.push(key);
      } catch (e) {
        nextErrors[key] = e instanceof Error ? e.message : String(e);
      }
    }
    // Re-read from the server rather than trusting the staged values locally
    // -- a write can be clamped, rejected mid-way, or transformed server-side,
    // and the option list should always reflect what the server has, not what
    // was asked for.
    try {
      const fresh = await api.discover(serverId);
      onReport(fresh);
    } catch {
      /* the SSE "discovery" listener (or the next manual re-scan) will catch up */
    }
    setEdits((prev) => {
      const next = { ...prev };
      for (const key of applied) delete next[key];
      return next;
    });
    setErrors(nextErrors);
    setApplying(false);
  };

  if (!connected) {
    return <Empty text="Connect to load settings." />;
  }
  if (!report) {
    return <Empty text="No discovery yet — use ↻ re-scan above." />;
  }
  if (!report.capabilities.options) {
    return <Empty text="This server did not report an options dump." />;
  }

  const readOnly = !report.capabilities.optionWrite;
  const dirtyCount = dirtyKeys.length;

  return (
    <div className="relative flex min-h-full flex-col">
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
            placeholder={`filter ${report.options.length} options…`}
            aria-label="Filter options"
            className="flex-1 bg-transparent font-mono text-[12.5px] outline-none"
          />
        </label>
        <span className="text-[11.5px]" style={{ color: "var(--color-text-faint)" }}>
          {readOnly
            ? "read-only — this server reported no write command"
            : "values read live from discovery · types inferred"}
        </span>
      </div>

      {sections.map(([name, sectionRows]) => {
        const visible = sectionRows.filter(matches);
        if (visible.length === 0) return null;
        return (
          <div key={name} className="px-4.5 pt-1 pb-0.5">
            <h3
              className="mt-3.5 mb-1 flex items-baseline gap-2 font-mono text-[10.5px] font-semibold tracking-[.13em] uppercase"
              style={{ color: "var(--color-text-faint)" }}
            >
              {name} <span style={{ color: "var(--color-accent)" }}>{visible.length}</span>
            </h3>
            <div>
              {visible.map((row) => (
                <OptionRow
                  key={row.key}
                  row={row}
                  staged={edits[row.key]}
                  error={errors[row.key]}
                  readOnly={readOnly}
                  onChange={(v) => setValue(row.key, v)}
                />
              ))}
            </div>
          </div>
        );
      })}

      {!readOnly && (
        <div
          className="sticky bottom-0 mt-4 border-t px-4.5 py-2.5"
          style={{
            borderColor: "var(--color-accent)",
            background: "var(--color-surface-raised)",
            display: dirtyCount > 0 ? "block" : "none",
          }}
        >
          <div className="flex items-center gap-3">
            <span className="font-mono text-xs font-semibold" style={{ color: "var(--color-accent)" }}>
              {dirtyCount} staged change{dirtyCount === 1 ? "" : "s"}
            </span>
            <span className="ml-auto flex gap-2">
              <button className="btn btn-ghost" onClick={discard} disabled={applying}>
                discard
              </button>
              <button className="btn btn-primary" onClick={apply} disabled={applying}>
                {applying ? "applying…" : "apply to server"}
              </button>
            </span>
          </div>
          <pre
            className="mt-2 overflow-x-auto rounded-md px-3 py-2 font-mono text-[11.5px] leading-[1.7]"
            style={{ background: "var(--color-surface-sunken)", color: "var(--color-text-muted)" }}
          >
            {dirtyKeys
              .map((key) => (
                <b key={key} className="block font-normal" style={{ color: "var(--color-text)" }}>
                  {buildCommand(report.writeTemplate, key, edits[key])}
                </b>
              ))}
          </pre>
        </div>
      )}
    </div>
  );
}

export function OptionRow({
  row,
  staged,
  error,
  readOnly,
  onChange,
}: {
  row: Row;
  staged: string | undefined;
  error: string | undefined;
  readOnly: boolean;
  onChange: (value: string) => void;
}) {
  const current = staged ?? row.value;
  const dirty = staged !== undefined && staged !== row.value;

  return (
    <div
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-0.5 rounded-md border-t px-2.5 py-2.5 first:border-t-0"
      style={{
        borderColor: "var(--color-border-soft)",
        background: dirty ? "var(--color-accent-soft)" : undefined,
      }}
    >
      <span className="font-mono text-[12.5px] font-medium">
        {row.meta?.label ?? row.key}
        {dirty && (
          <span className="ml-1.5 align-top text-[9px]" style={{ color: "var(--color-accent)" }}>
            ●
          </span>
        )}
        {row.meta?.requiresRestart && <span className="badge-restart">restart</span>}
      </span>

      <span className="row-span-2 flex items-center gap-2 [font-variant-numeric:tabular-nums]">
        {dirty && (
          <span
            className="font-mono text-[10.5px] line-through"
            style={{ color: "var(--color-text-faint)" }}
          >
            {row.value}
          </span>
        )}
        <Control row={row} value={current} readOnly={readOnly} onChange={onChange} />
      </span>

      <span className="max-w-[62ch] text-[11.5px]" style={{ color: "var(--color-text-muted)" }}>
        {row.meta?.description ?? "Uncurated option — no metadata yet, edited as raw text."}
      </span>

      {error && (
        <span className="col-span-2 text-[11px]" style={{ color: "var(--color-danger)" }}>
          {error}
        </span>
      )}
    </div>
  );
}

export function Control({
  row,
  value,
  readOnly,
  onChange,
}: {
  row: Row;
  value: string;
  readOnly: boolean;
  onChange: (value: string) => void;
}) {
  if (readOnly) {
    return <span className="font-mono text-[12.5px]">{value}</span>;
  }

  if (row.type === "boolean") {
    const checked = value === "true";
    return (
      <button
        role="switch"
        aria-checked={checked}
        aria-label={row.meta?.label ?? row.key}
        className="switch"
        onClick={() => onChange(checked ? "false" : "true")}
      />
    );
  }

  if (row.type === "integer" || row.type === "number") {
    const step = row.type === "integer" ? 1 : 0.1;
    const bump = (delta: number) => {
      const n = (row.type === "integer" ? parseInt(value, 10) : parseFloat(value)) || 0;
      let next = n + delta;
      if (row.meta?.min !== undefined) next = Math.max(row.meta.min, next);
      if (row.meta?.max !== undefined) next = Math.min(row.meta.max, next);
      // Repeated float addition drifts ("70.30000000000001") -- the staging
      // bar's whole premise is that the preview is the exact string sent, so
      // that string has to be rounded before it's built, not after.
      onChange(row.type === "integer" ? String(Math.round(next)) : String(Number(next.toFixed(2))));
    };
    return (
      <span
        className="flex items-center rounded-md border"
        style={{ background: "var(--color-surface-sunken)" }}
      >
        <button
          type="button"
          aria-label="decrease"
          className="cursor-pointer px-2.5 py-1 font-mono text-[13px]"
          style={{ color: "var(--color-text-muted)" }}
          onClick={() => bump(-step)}
        >
          −
        </button>
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          inputMode="decimal"
          className="w-13 bg-transparent text-center font-mono text-[12.5px] outline-none"
        />
        <button
          type="button"
          aria-label="increase"
          className="cursor-pointer px-2.5 py-1 font-mono text-[13px]"
          style={{ color: "var(--color-text-muted)" }}
          onClick={() => bump(step)}
        >
          +
        </button>
      </span>
    );
  }

  if (row.meta?.options) {
    return (
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-auto rounded-md border px-2 py-1 font-mono text-[12.5px] outline-none"
        style={{ background: "var(--color-surface-sunken)" }}
      >
        {row.meta.options.map((opt) => (
          <option key={opt} value={opt}>
            {opt}
          </option>
        ))}
      </select>
    );
  }

  return (
    <input
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="field w-50 font-mono text-[12.5px]"
    />
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
