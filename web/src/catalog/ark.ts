// Representative RCON surface -- verify against your server; not exhaustive.
import type { GameTemplate } from "./types";

export const arkTemplate: GameTemplate = {
  id: "ark",
  name: "ARK: Survival Ascended",
  defaultPort: 27020,
  capabilities: { options: false, optionWrite: false, commands: true },
  caveats: ["Settings live in GameUserSettings.ini, not readable over RCON."],
  options: [],
  commands: [
    { name: "ListPlayers", description: "List connected players and their Steam/EOS IDs." },
    { name: "Broadcast", description: "Send a message to all players.", usage: "Broadcast <message>" },
    { name: "SaveWorld", description: "Force an immediate world save." },
    {
      name: "KickPlayer",
      description: "Disconnect a player by their platform ID.",
      usage: "KickPlayer <playerid>",
    },
    {
      name: "BanPlayer",
      description: "Ban a player by their platform ID.",
      usage: "BanPlayer <playerid>",
    },
    { name: "DoExit", description: "Save and shut down the server immediately." },
    {
      name: "GetGameLog",
      description: "Retrieve the recent server game log (joins, deaths, tribe events).",
    },
    {
      name: "ServerChat",
      description: "Send a server-wide chat message.",
      usage: "ServerChat <message>",
    },
  ],
};
