import type { GameTemplate } from "./types";

/** Vanilla gamerule descriptions for Minecraft: Java Edition, plus a
 * representative admin command set.
 *
 * Sourced from the vanilla gamerule set as of 1.20; a modded or older server
 * may expose gamerules this table has no entry for (or omit ones it lists).
 * Either way a live option still renders -- see zomboid.ts's header comment
 * for why an out-of-date table is safe here.
 *
 * This is the single source of truth for Minecraft enrichment: `metaFor` in
 * enrich/index.ts reads straight from `options` below, so the live Settings
 * panel and this catalog can never drift apart.
 */
export const minecraftTemplate: GameTemplate = {
  id: "minecraft",
  name: "Minecraft: Java Edition",
  defaultPort: 25575,
  capabilities: { options: true, optionWrite: true, commands: true },
  writeTemplate: "gamerule {key} {value}",
  caveats: [
    "gamerule only covers vanilla rules read at query time -- data pack- or plugin-added rules can be missing from a given server's list.",
    "server.properties settings (difficulty, view-distance, ...) are not gamerules and are not covered here; changing most of them still requires a restart.",
  ],
  options: [
    // -- World --------------------------------------------------------------
    {
      key: "doDaylightCycle",
      example: "true",
      section: "World",
      description: "Advance time normally; disable to freeze the day/night cycle.",
    },
    {
      key: "doWeatherCycle",
      example: "true",
      section: "World",
      description: "Allow weather to change on its own.",
    },
    {
      key: "doFireTick",
      example: "true",
      section: "World",
      description: "Let fire spread and burn out over time.",
    },
    {
      key: "randomTickSpeed",
      example: "3",
      section: "World",
      description:
        "Random block updates applied per loaded chunk section each tick (crop growth, leaf decay, ...).",
      min: 0,
    },
    {
      key: "spawnRadius",
      example: "10",
      section: "World",
      description: "Blocks around the world spawn point where new players can appear.",
      min: 0,
    },
    {
      key: "maxEntityCramming",
      example: "24",
      section: "World",
      description: "Entities packed into one block before they take suffocation damage.",
      min: 0,
    },

    // -- Mobs --------------------------------------------------------------
    {
      key: "doMobSpawning",
      example: "true",
      section: "Mobs",
      description: "Allow mobs to spawn naturally.",
    },
    {
      key: "doInsomnia",
      example: "true",
      section: "Mobs",
      description: "Allow phantoms to spawn from insomnia (players who haven't slept).",
    },
    {
      key: "doPatrolSpawning",
      example: "true",
      section: "Mobs",
      description: "Allow pillager patrols to spawn.",
    },
    {
      key: "doTraderSpawning",
      example: "true",
      section: "Mobs",
      description: "Allow wandering traders to spawn.",
    },
    {
      key: "mobGriefing",
      example: "true",
      section: "Mobs",
      description: "Let mobs change blocks (creeper craters, enderman pickups, ...).",
    },
    {
      key: "universalAnger",
      example: "false",
      section: "Mobs",
      description:
        "Neutral mobs angered by any player attack anyone nearby, not just the attacker.",
    },
    {
      key: "forgiveDeadPlayers",
      example: "true",
      section: "Mobs",
      description: "Stop neutral mobs from staying angry at a player who has died.",
    },

    // -- Player --------------------------------------------------------------
    {
      key: "keepInventory",
      example: "false",
      section: "Player",
      description: "Players keep their inventory and XP on death.",
    },
    {
      key: "naturalRegeneration",
      example: "true",
      section: "Player",
      description: "Players regenerate health automatically when hunger allows.",
    },
    {
      key: "doImmediateRespawn",
      example: "false",
      section: "Player",
      description: "Skip the death screen and respawn immediately.",
    },
    {
      key: "fallDamage",
      example: "true",
      section: "Player",
      description: "Players and mobs take damage from falling.",
    },
    {
      key: "fireDamage",
      example: "true",
      section: "Player",
      description: "Players and mobs take damage from fire and lava.",
    },
    {
      key: "drowningDamage",
      example: "true",
      section: "Player",
      description: "Players and mobs take damage from drowning.",
    },
    {
      key: "freezeDamage",
      example: "true",
      section: "Player",
      description: "Players and mobs take damage from powder snow.",
    },
    {
      key: "playersSleepingPercentage",
      example: "100",
      section: "Player",
      description: "Percentage of online players who must sleep to skip the night.",
      min: 0,
      max: 100,
    },

    // -- Admin & Feedback --------------------------------------------------------------
    {
      key: "announceAdvancements",
      example: "true",
      section: "Admin & Feedback",
      description: "Broadcast advancement completions in chat.",
    },
    {
      key: "commandBlockOutput",
      example: "true",
      section: "Admin & Feedback",
      description: "Broadcast command block output to admins.",
    },
    {
      key: "logAdminCommands",
      example: "true",
      section: "Admin & Feedback",
      description: "Log admin command usage to the console.",
    },
    {
      key: "sendCommandFeedback",
      example: "true",
      section: "Admin & Feedback",
      description: "Show command feedback in chat.",
    },
    {
      key: "showDeathMessages",
      example: "true",
      section: "Admin & Feedback",
      description: "Broadcast a chat line when a player dies.",
    },
    {
      key: "reducedDebugInfo",
      example: "false",
      section: "Admin & Feedback",
      description: "Hide detailed information from the client's F3 debug screen.",
    },
    {
      key: "maxCommandChainLength",
      example: "65536",
      section: "Admin & Feedback",
      description: "Maximum commands a single command block chain can execute.",
      min: 1,
    },
    {
      key: "doLimitedCrafting",
      example: "false",
      section: "Admin & Feedback",
      description: "Require a recipe to be unlocked before it can be crafted.",
    },
    {
      key: "disableElytraMovementCheck",
      example: "false",
      section: "Admin & Feedback",
      description: "Disable the anti-cheat check for fast elytra movement.",
    },
  ],
  commands: [
    { name: "list", description: "List connected players." },
    { name: "say", description: "Broadcast a message in chat.", usage: "say <message>" },
    { name: "kick", description: "Disconnect a player.", usage: "kick <player> [reason]" },
    { name: "ban", description: "Ban a player.", usage: "ban <player> [reason]" },
    { name: "pardon", description: "Remove a ban.", usage: "pardon <player>" },
    { name: "op", description: "Grant operator status.", usage: "op <player>" },
    { name: "deop", description: "Revoke operator status.", usage: "deop <player>" },
    { name: "save-all", description: "Force a world save." },
    { name: "stop", description: "Save and shut down the server." },
    {
      name: "gamemode",
      description: "Change a player's game mode.",
      usage: "gamemode <survival|creative|adventure|spectator> <player>",
    },
  ],
};
