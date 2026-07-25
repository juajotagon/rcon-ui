// Representative RCON surface -- verify against your server; not exhaustive.
import type { GameTemplate } from "./types";

export const sevenDaysToDieTemplate: GameTemplate = {
  id: "sevendaystodie",
  name: "7 Days to Die",
  defaultPort: 8081,
  capabilities: { options: false, optionWrite: false, commands: true },
  caveats: ["7DTD RCON derives from its telnet console; behavior can differ from Source RCON."],
  options: [],
  commands: [
    { name: "listplayers", description: "List connected players with IDs and ping." },
    { name: "say", description: "Broadcast a message to all players.", usage: "say <message>" },
    { name: "kick", description: "Disconnect a player.", usage: "kick <name> [reason]" },
    {
      name: "ban add",
      description: "Ban a player for a duration.",
      usage: "ban add <name> <duration> [reason]",
    },
    { name: "saveworld", description: "Force an immediate world save." },
    { name: "settime", description: "Set the in-game time.", usage: "settime <ticks>" },
    { name: "shutdown", description: "Save and shut down the server." },
    { name: "gettime", description: "Print the current in-game day and time." },
  ],
};
