import { useEffect, useState } from "react";
import { api, type Server } from "../api";

/** ServerDialog adds or edits a server profile.
 *
 * Progressive disclosure: name, address and password are all that is needed to
 * connect, so protocol and group sit behind "Advanced". Someone adding their
 * first Minecraft server should not have to think about protocol dialects.
 */
export function ServerDialog({
  server,
  protocols,
  onClose,
  onSaved,
}: {
  server: Server | null; // null means "create"
  protocols: string[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(server?.name ?? "");
  const [addr, setAddr] = useState(server?.addr ?? "");
  const [password, setPassword] = useState("");
  const [group, setGroup] = useState(server?.group ?? "");
  const [protocol, setProtocol] = useState(server?.protocol ?? "source");
  const [tls, setTls] = useState(server?.tls ?? false);
  const [serverName, setServerName] = useState(server?.serverName ?? "");
  // Open Advanced from the start when the profile already uses it, so the
  // settings being edited are visible rather than silently applied.
  const [advanced, setAdvanced] = useState(Boolean(server?.tls || server?.group));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const save = async () => {
    setBusy(true);
    setError(null);
    try {
      if (server) {
        await api.updateServer(server.id, {
          name,
          addr,
          group,
          protocol,
          tls,
          serverName: tls ? serverName : "",
          // Omitted rather than sent empty, so an edit that does not touch the
          // password leaves the stored one alone.
          ...(password ? { password } : {}),
        });
      } else {
        await api.createServer({
          name,
          addr,
          password,
          group,
          protocol,
          tls,
          serverName: tls ? serverName : "",
        });
      }
      onSaved();
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const valid = name.trim() && addr.trim() && (server || password);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-md rounded-lg border shadow-xl"
        style={{ background: "var(--color-surface-raised)" }}
      >
        <h2 className="border-b px-4 py-3 text-sm font-semibold">
          {server ? "Edit server" : "Add server"}
        </h2>

        <div className="space-y-3 p-4">
          <Field label="Name">
            <input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Minecraft server"
              className="field"
            />
          </Field>

          <Field label="Address" hint="host:port — the RCON port, not the game port">
            <input
              value={addr}
              onChange={(e) => setAddr(e.target.value)}
              placeholder="mc.example.com:25575"
              className="field font-mono"
            />
          </Field>

          <Field label={server ? "Password (leave blank to keep)" : "RCON password"}>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={server ? "unchanged" : ""}
              className="field font-mono"
            />
          </Field>

          <button
            onClick={() => setAdvanced(!advanced)}
            className="text-xs underline-offset-2 hover:underline"
            style={{ color: "var(--color-text-muted)" }}
          >
            {advanced ? "Hide" : "Show"} advanced
          </button>

          {advanced && (
            <div className="space-y-3 border-t pt-3">
              <Field label="Group" hint="Optional. Groups servers in the sidebar.">
                <input
                  value={group}
                  onChange={(e) => setGroup(e.target.value)}
                  placeholder="home"
                  className="field"
                />
              </Field>

              <Field label="Protocol">
                <select
                  value={protocol}
                  onChange={(e) => setProtocol(e.target.value)}
                  className="field"
                >
                  {protocols.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
              </Field>

              <label className="flex items-center gap-2 text-xs font-medium">
                <input
                  type="checkbox"
                  checked={tls}
                  onChange={(e) => setTls(e.target.checked)}
                />
                Connect through TLS
                <span className="font-normal" style={{ color: "var(--color-text-muted)" }}>
                  for servers behind a TLS-terminating proxy
                </span>
              </label>

              {tls && (
                <Field
                  label="TLS server name"
                  hint="Optional. Only needed when the certificate names a different host than the address."
                >
                  <input
                    value={serverName}
                    onChange={(e) => setServerName(e.target.value)}
                    placeholder="zomboid.example.com"
                    className="field font-mono"
                  />
                </Field>
              )}
            </div>
          )}

          {error && (
            <p className="text-xs" style={{ color: "var(--color-danger)" }}>
              {error}
            </p>
          )}
        </div>

        <div className="flex justify-end gap-2 border-t px-4 py-3">
          <button onClick={onClose} className="btn btn-ghost">
            Cancel
          </button>
          <button onClick={save} disabled={!valid || busy} className="btn btn-primary">
            {busy ? "Saving…" : "Save"}
          </button>
        </div>
      </div>
    </div>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-medium">{label}</span>
      {children}
      {hint && (
        <span className="mt-1 block text-[11px]" style={{ color: "var(--color-text-muted)" }}>
          {hint}
        </span>
      )}
    </label>
  );
}
