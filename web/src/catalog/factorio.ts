// Representative RCON surface -- verify against your server; not exhaustive.
import type { GameTemplate } from "./types";

export const factorioTemplate: GameTemplate = {
  id: "factorio",
  name: "Factorio",
  defaultPort: 27015,
  capabilities: { options: false, optionWrite: false, commands: true },
  caveats: ["Factorio RCON executes server Lua -- /c and /sc run arbitrary code. No settings dump."],
  options: [],
  commands: [
    { name: "/players", description: "List connected players." },
    { name: "/save", description: "Save the game, optionally to a named slot.", usage: "/save [name]" },
    { name: "/ban", description: "Ban a player.", usage: "/ban <player> [reason]" },
    { name: "/kick", description: "Disconnect a player.", usage: "/kick <player> [reason]" },
    {
      name: "/whitelist",
      description: "Manage the whitelist.",
      usage: "/whitelist <add|remove|get> [player]",
    },
    { name: "/server-save", description: "Force an immediate autosave." },
    {
      name: "/c",
      description: "Execute Lua immediately and print the returned value.",
      usage: "/c <lua expression>",
    },
    {
      name: "/sc",
      description: "Execute Lua silently, with no console echo.",
      usage: "/sc <lua expression>",
    },
  ],
};
