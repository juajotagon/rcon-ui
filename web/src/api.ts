// Client for the rcon-ui daemon.
//
// Deliberately free of any knowledge of whether it is talking to a self-hosted
// daemon or the desktop shell: the desktop build runs the same HTTP server on
// loopback and points a webview at it, so there is one transport and no
// branching. Anything that would need a desktop-only binding belongs somewhere
// else.

export type Status = "disconnected" | "connecting" | "connected" | "auth_failed";

export interface Server {
  id: string;
  name: string;
  protocol: string;
  addr: string;
  group?: string;
  createdAt: string;
  updatedAt: string;
  status: Status;
  // The last fingerprint discovery settled on for this server, persisted so
  // the rail can show a game chip before a connection (and thus a fresh
  // discovery pass) exists.
  game?: string;
}

export interface RconEvent {
  seq: number;
  profileId: string;
  source: string;
  stream: "command" | "response" | "status" | "error" | "discovery";
  line: string;
  at: string;
}

export interface DiscoveredCommand {
  name: string;
  description: string;
  usage: string;
}

export interface DiscoveredOption {
  key: string;
  value: string;
  type: "boolean" | "integer" | "number" | "string";
}

export interface DiscoveryReport {
  fingerprint: "zomboid" | "minecraft" | "unknown";
  label: string;
  at: string;
  capabilities: { commands: boolean; options: boolean; optionWrite: boolean };
  // Substitutes {key}/{value} literally; empty when optionWrite is false, in
  // which case there is no write path to build a command string from.
  writeTemplate: string;
  commands: DiscoveredCommand[];
  options: DiscoveredOption[];
}

export interface Macro {
  id: string;
  serverId?: string;
  name: string;
  command: string;
  position: number;
}

export interface HistoryEntry {
  id: number;
  serverId: string;
  command: string;
  at: string;
}

// The desktop shell injects a per-launch token, because a loopback-bound API
// with no credential is reachable by any other local process and by any web
// page via DNS rebinding.
declare global {
  interface Window {
    __RCON_UI_TOKEN__?: string;
  }
}

let cachedToken: string | undefined;

/** token resolves the API credential, if this build needs one.
 *
 * The desktop shell navigates to `/?access_token=…` because a webview cannot be
 * given custom headers for its initial navigation. It is read once, kept in
 * memory, and stripped from the visible URL — leaving it in the address bar
 * would put it in any screenshot the user shares, and in the Referer of any
 * link they follow.
 *
 * The self-hosted build usually has no token at all, in which case everything
 * here is a no-op.
 */
function token(): string | undefined {
  if (cachedToken) return cachedToken;

  if (window.__RCON_UI_TOKEN__) {
    cachedToken = window.__RCON_UI_TOKEN__;
    return cachedToken;
  }

  const params = new URLSearchParams(window.location.search);
  const fromUrl = params.get("access_token");
  if (fromUrl) {
    cachedToken = fromUrl;
    params.delete("access_token");
    const query = params.toString();
    history.replaceState(null, "", window.location.pathname + (query ? `?${query}` : ""));
    return cachedToken;
  }
  return undefined;
}

/** withToken appends the token as a query parameter.
 *
 * Not laziness: EventSource cannot set request headers, so a header-only
 * scheme would leave the event stream unauthenticatable from the browser. The
 * same parameter is used everywhere for consistency.
 */
export function withToken(path: string): string {
  const t = token();
  if (!t) return path;
  return path + (path.includes("?") ? "&" : "?") + "access_token=" + encodeURIComponent(t);
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";

  const t = token();
  if (t) headers["Authorization"] = "Bearer " + t;

  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (!res.ok) {
    // The daemon reports failures as {"error": "..."}; fall back to the status
    // text when something else produced the response (a proxy, say).
    let message = res.statusText;
    try {
      const data = await res.json();
      if (data?.error) message = data.error;
    } catch {
      /* keep statusText */
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  listServers: () => request<Server[]>("GET", "/api/servers"),

  createServer: (input: {
    name: string;
    addr: string;
    password: string;
    protocol?: string;
    group?: string;
  }) => request<Server>("POST", "/api/servers", input),

  updateServer: (
    id: string,
    input: { name?: string; addr?: string; protocol?: string; group?: string; password?: string },
  ) => request<Server>("PATCH", `/api/servers/${id}`, input),

  deleteServer: (id: string) => request<void>("DELETE", `/api/servers/${id}`),

  connect: (id: string) => request<{ status: Status }>("POST", `/api/servers/${id}/connect`),
  disconnect: (id: string) => request<{ status: Status }>("POST", `/api/servers/${id}/disconnect`),

  execute: (id: string, command: string) =>
    request<{ response: string }>("POST", `/api/servers/${id}/execute`, { command }),

  // Callers distinguish "never scanned" (404) from a real failure by checking
  // ApiError.status themselves -- that boundary is meaningful here (it is the
  // bottom rung of the degradation ladder, not an error state) so it is not
  // collapsed inside the client.
  discovery: (id: string) => request<DiscoveryReport>("GET", `/api/servers/${id}/discovery`),

  discover: (id: string) => request<DiscoveryReport>("POST", `/api/servers/${id}/discover`),

  history: (id: string, limit = 100) =>
    request<HistoryEntry[]>("GET", `/api/servers/${id}/history?limit=${limit}`),

  listMacros: (serverId: string) =>
    request<Macro[]>("GET", `/api/macros?serverId=${encodeURIComponent(serverId)}`),

  createMacro: (input: { serverId?: string; name: string; command: string; position?: number }) =>
    request<Macro>("POST", "/api/macros", input),

  deleteMacro: (id: string) => request<void>(`DELETE`, `/api/macros/${id}`),

  protocols: () => request<string[]>("GET", "/api/protocols"),
};
