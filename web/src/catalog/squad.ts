// Representative RCON surface -- verify against your server; not exhaustive.
import type { GameTemplate } from "./types";

export const squadTemplate: GameTemplate = {
  id: "squad",
  name: "Squad",
  defaultPort: 21114,
  capabilities: { options: false, optionWrite: false, commands: true },
  caveats: ["Admin command surface; no settings enumeration."],
  options: [],
  commands: [
    { name: "ListPlayers", description: "List connected players, teams, and Steam IDs." },
    {
      name: "AdminBroadcast",
      description: "Broadcast a message to all players.",
      usage: "AdminBroadcast <message>",
    },
    {
      name: "AdminKick",
      description: "Disconnect a player.",
      usage: "AdminKick <playerid> [reason]",
    },
    {
      name: "AdminBan",
      description: "Ban a player for a duration.",
      usage: "AdminBan <playerid> <duration> [reason]",
    },
    {
      name: "AdminChangeMap",
      description: "Switch to a different map/layer.",
      usage: "AdminChangeMap <layer>",
    },
    { name: "AdminRestartMatch", description: "Restart the current match immediately." },
    { name: "ShowNextMap", description: "Print the next map in rotation." },
  ],
};
