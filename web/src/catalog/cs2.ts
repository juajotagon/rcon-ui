// Representative RCON surface -- verify against your server; not exhaustive.
import type { GameTemplate } from "./types";

/** Counter-Strike 2's RCON is the Source engine's original console-over-TCP:
 * every cvar and every console command is fair game, which is exactly why
 * there's no enumerable settings dump -- "options" here would just be an
 * arbitrary slice of thousands of cvars with no canonical list to draw from.
 */
export const cs2Template: GameTemplate = {
  id: "cs2",
  name: "Counter-Strike 2",
  defaultPort: 27015,
  capabilities: { options: false, optionWrite: false, commands: true },
  caveats: ["Configuration is cvars set via console, not enumerable over RCON."],
  options: [],
  commands: [
    { name: "status", description: "List server status and connected players." },
    { name: "say", description: "Broadcast a chat message to all players.", usage: "say <message>" },
    {
      name: "mp_restartgame",
      description: "Restart the round after a delay in seconds.",
      usage: "mp_restartgame <seconds>",
    },
    { name: "changelevel", description: "Switch to a different map.", usage: "changelevel <mapname>" },
    { name: "bot_add", description: "Add a bot to a team.", usage: "bot_add [T|CT]" },
    { name: "sv_cheats", description: "Toggle cheat-protected commands.", usage: "sv_cheats <0|1>" },
    { name: "kick", description: "Disconnect a player by name or slot.", usage: "kick <name|#userid>" },
    { name: "mp_warmup_start", description: "Force the warmup period to begin." },
    {
      name: "host_workshop_map",
      description: "Load a Workshop map by its published file ID.",
      usage: "host_workshop_map <workshop_id>",
    },
  ],
};
