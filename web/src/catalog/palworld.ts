// Representative RCON surface -- verify against your server; not exhaustive.
import type { GameTemplate } from "./types";

export const palworldTemplate: GameTemplate = {
  id: "palworld",
  name: "Palworld",
  defaultPort: 25575,
  capabilities: { options: false, optionWrite: false, commands: true },
  caveats: [
    "Fixed command set; no settings dump.",
    "A separate REST API covers more (player management, server metrics).",
  ],
  options: [],
  commands: [
    { name: "Info", description: "Print server name and version." },
    { name: "ShowPlayers", description: "List connected players with UID and SteamID." },
    { name: "Broadcast", description: "Send a message to all players.", usage: "Broadcast <message>" },
    {
      name: "KickPlayer",
      description: "Disconnect a player by SteamID.",
      usage: "KickPlayer <steamid>",
    },
    { name: "BanPlayer", description: "Ban a player by SteamID.", usage: "BanPlayer <steamid>" },
    { name: "Save", description: "Force an immediate world save." },
    {
      name: "Shutdown",
      description: "Schedule a graceful shutdown with a countdown message.",
      usage: "Shutdown <seconds> <message>",
    },
    { name: "DoExit", description: "Shut down the server immediately without a countdown." },
    {
      name: "TeleportToPlayer",
      description: "Teleport the admin to a player's location.",
      usage: "TeleportToPlayer <steamid>",
    },
  ],
};
