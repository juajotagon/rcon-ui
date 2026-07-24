import type { EnrichmentTable } from "./types";

/** Vanilla gamerule descriptions for Minecraft: Java Edition.
 *
 * Sourced from the vanilla gamerule set as of 1.20; a modded or older server
 * may expose gamerules this table has no entry for (or omit ones it lists).
 * Either way the option still renders -- see zomboid.ts's header comment for
 * why an out-of-date table is safe here.
 */
export const minecraftOptions: EnrichmentTable = {
  // -- World --------------------------------------------------------------
  doDaylightCycle: {
    section: "World",
    description: "Advance time normally; disable to freeze the day/night cycle.",
  },
  doWeatherCycle: { section: "World", description: "Allow weather to change on its own." },
  doFireTick: { section: "World", description: "Let fire spread and burn out over time." },
  randomTickSpeed: {
    section: "World",
    description:
      "Random block updates applied per loaded chunk section each tick (crop growth, leaf decay, ...).",
    min: 0,
  },
  spawnRadius: {
    section: "World",
    description: "Blocks around the world spawn point where new players can appear.",
    min: 0,
  },
  maxEntityCramming: {
    section: "World",
    description: "Entities packed into one block before they take suffocation damage.",
    min: 0,
  },

  // -- Mobs --------------------------------------------------------------
  doMobSpawning: { section: "Mobs", description: "Allow mobs to spawn naturally." },
  doInsomnia: {
    section: "Mobs",
    description: "Allow phantoms to spawn from insomnia (players who haven't slept).",
  },
  doPatrolSpawning: { section: "Mobs", description: "Allow pillager patrols to spawn." },
  doTraderSpawning: { section: "Mobs", description: "Allow wandering traders to spawn." },
  mobGriefing: {
    section: "Mobs",
    description: "Let mobs change blocks (creeper craters, enderman pickups, ...).",
  },
  universalAnger: {
    section: "Mobs",
    description:
      "Neutral mobs angered by any player attack anyone nearby, not just the attacker.",
  },
  forgiveDeadPlayers: {
    section: "Mobs",
    description: "Stop neutral mobs from staying angry at a player who has died.",
  },

  // -- Player --------------------------------------------------------------
  keepInventory: {
    section: "Player",
    description: "Players keep their inventory and XP on death.",
  },
  naturalRegeneration: {
    section: "Player",
    description: "Players regenerate health automatically when hunger allows.",
  },
  doImmediateRespawn: {
    section: "Player",
    description: "Skip the death screen and respawn immediately.",
  },
  fallDamage: { section: "Player", description: "Players and mobs take damage from falling." },
  fireDamage: {
    section: "Player",
    description: "Players and mobs take damage from fire and lava.",
  },
  drowningDamage: { section: "Player", description: "Players and mobs take damage from drowning." },
  freezeDamage: {
    section: "Player",
    description: "Players and mobs take damage from powder snow.",
  },
  playersSleepingPercentage: {
    section: "Player",
    description: "Percentage of online players who must sleep to skip the night.",
    min: 0,
    max: 100,
  },

  // -- Admin & Feedback --------------------------------------------------------------
  announceAdvancements: {
    section: "Admin & Feedback",
    description: "Broadcast advancement completions in chat.",
  },
  commandBlockOutput: {
    section: "Admin & Feedback",
    description: "Broadcast command block output to admins.",
  },
  logAdminCommands: {
    section: "Admin & Feedback",
    description: "Log admin command usage to the console.",
  },
  sendCommandFeedback: {
    section: "Admin & Feedback",
    description: "Show command feedback in chat.",
  },
  showDeathMessages: {
    section: "Admin & Feedback",
    description: "Broadcast a chat line when a player dies.",
  },
  reducedDebugInfo: {
    section: "Admin & Feedback",
    description: "Hide detailed information from the client's F3 debug screen.",
  },
  maxCommandChainLength: {
    section: "Admin & Feedback",
    description: "Maximum commands a single command block chain can execute.",
    min: 1,
  },
  doLimitedCrafting: {
    section: "Admin & Feedback",
    description: "Require a recipe to be unlocked before it can be crafted.",
  },
  disableElytraMovementCheck: {
    section: "Admin & Feedback",
    description: "Disable the anti-cheat check for fast elytra movement.",
  },
};
