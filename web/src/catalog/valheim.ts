// Representative RCON surface -- verify against your server; not exhaustive.
import type { GameTemplate } from "./types";

/** Valheim ships with no RCON server at all -- everything below only exists
 * once a third-party mod adds one, so capabilities are all false here (there
 * is nothing this build can rely on out of the box) even though a couple of
 * commonly-modded commands are listed for reference.
 */
export const valheimTemplate: GameTemplate = {
  id: "valheim",
  name: "Valheim",
  defaultPort: 2456,
  capabilities: { options: false, optionWrite: false, commands: false },
  caveats: [
    "Valheim has NO native RCON -- it requires a server mod (e.g. a BepInEx RCON plugin); command set depends on the mod. Listed for reference.",
  ],
  options: [],
  commands: [
    { name: "save", description: "Force an immediate world save (requires a server-side RCON mod)." },
    { name: "kick", description: "Disconnect a player (mod-dependent syntax).", usage: "kick <name>" },
    { name: "ban", description: "Ban a player (mod-dependent syntax).", usage: "ban <name>" },
  ],
};
