// Representative RCON surface -- verify against your server; not exhaustive.
import type { GameTemplate } from "./types";

export const rustTemplate: GameTemplate = {
  id: "rust",
  name: "Rust",
  defaultPort: 28016,
  capabilities: { options: false, optionWrite: false, commands: true },
  caveats: [
    "Rust RCON is typically a WebSocket protocol (RCON.web); this client speaks the TCP Source variant -- verify your server exposes it.",
    "Settings are convars, not enumerable.",
  ],
  options: [],
  commands: [
    { name: "status", description: "List server status and player count." },
    { name: "say", description: "Broadcast a message to all players.", usage: "say <message>" },
    {
      name: "serverinfo",
      description: "Print server info as JSON (players, uptime, framerate).",
    },
    { name: "playerlist", description: "List connected players with SteamIDs." },
    { name: "ban", description: "Ban a player by SteamID.", usage: "ban <steamid> [reason]" },
    { name: "kick", description: "Disconnect a connected player.", usage: "kick <steamid> [reason]" },
    { name: "save", description: "Force an immediate world save." },
    {
      name: "teleport",
      description: "Teleport a player to coordinates.",
      usage: "teleport <name> <x> <y> <z>",
    },
    {
      name: "weather.fog",
      description: "Set fog weight for the day/night cycle.",
      usage: "weather.fog <0-1>",
    },
  ],
};
