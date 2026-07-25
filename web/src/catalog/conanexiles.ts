// Representative RCON surface -- verify against your server; not exhaustive.
import type { GameTemplate } from "./types";

export const conanExilesTemplate: GameTemplate = {
  id: "conanexiles",
  name: "Conan Exiles",
  defaultPort: 25575,
  capabilities: { options: false, optionWrite: false, commands: true },
  caveats: [
    "Command surface only; settings in ServerSettings.ini.",
    "Public documentation of Conan Exiles' RCON command set is sparse; verify against your server build.",
  ],
  options: [],
  commands: [
    { name: "listplayers", description: "List connected players and character names." },
    { name: "kick", description: "Disconnect a player.", usage: "kick <name>" },
    { name: "ban", description: "Ban a player.", usage: "ban <name>" },
    { name: "save", description: "Force an immediate world save." },
    { name: "broadcast", description: "Send a server-wide message.", usage: "broadcast <message>" },
  ],
};
