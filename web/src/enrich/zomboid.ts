import type { EnrichmentTable } from "./types";

/** Curated servertest.ini keys for Project Zomboid.
 *
 * Deliberately not exhaustive -- Zomboid's option dump runs past a hundred
 * keys across mod-added entries, and any list here will lag the game. Keys
 * missing from this table (Mods, Map, ServerPlayerID, ...) still show up via
 * `showoptions`, just under "Other" with no description, so an outdated
 * table degrades to plain text rather than hiding anything.
 */
export const zomboidOptions: EnrichmentTable = {
  // -- Server --------------------------------------------------------------
  Public: { section: "Server", description: "List this server in the public in-game browser." },
  PublicName: { section: "Server", description: "Name shown in the server browser." },
  PublicDescription: {
    section: "Server",
    description: "Short blurb shown in the server browser listing.",
  },
  MaxPlayers: {
    section: "Server",
    description: "Player slots. Read at startup; takes effect after the next restart.",
    min: 1,
    max: 100,
    requiresRestart: true,
  },
  Open: { section: "Server", description: "Allow joining without whitelist approval." },
  PingLimit: {
    section: "Server",
    description: "Kick players whose ping stays above this (ms).",
    min: 0,
    max: 999,
  },
  AutoCreateUserInWhiteList: {
    section: "Server",
    description: "Automatically whitelist a username the first time it logs in.",
  },
  DisplayUserName: {
    section: "Server",
    description: "Show player names above their characters.",
  },
  ServerWelcomeMessage: {
    section: "Server",
    description: "Message shown in chat when a player joins.",
  },
  LoginQueueEnabled: {
    section: "Server",
    description: "Queue joins when the server is full instead of rejecting them.",
  },
  LoginQueueConnectTimeout: {
    section: "Server",
    description: "Seconds a queued connection is held before it's dropped.",
    min: 0,
  },

  // -- Gameplay --------------------------------------------------------------
  PVP: {
    section: "Gameplay",
    description: "Player-versus-player damage anywhere outside safe zones.",
  },
  PVPMeleeDamageModifier: {
    section: "Gameplay",
    description: "Melee damage multiplier (%) applied to PVP hits.",
    min: 0,
    max: 500,
  },
  PVPFirearmDamageModifier: {
    section: "Gameplay",
    description: "Firearm damage multiplier (%) applied to PVP hits.",
    min: 0,
    max: 500,
  },
  PauseEmpty: { section: "Gameplay", description: "Freeze world time while nobody is online." },
  AnnounceDeath: {
    section: "Gameplay",
    description: "Broadcast a chat line when a player dies.",
  },
  SpeedLimit: { section: "Gameplay", description: "Vehicle speed cap.", min: 0, max: 200 },
  KnockedDownAllowed: {
    section: "Gameplay",
    description: "Players can be knocked down in melee.",
  },
  SleepAllowed: { section: "Gameplay", description: "Players can sleep in beds to pass time." },
  SleepNeeded: {
    section: "Gameplay",
    description: "Players suffer stat penalties if they don't sleep.",
  },
  PlayerRespawnWithSelf: {
    section: "Gameplay",
    description: "New character spawns with the same inventory as the last death.",
  },
  PlayerRespawnWithOther: {
    section: "Gameplay",
    description: "New character can loot their own corpse's items from another player.",
  },

  // -- Safehouses --------------------------------------------------------------
  SafehouseAllowTrepass: {
    section: "Safehouses",
    description: "Non-members may enter claimed safehouses.",
  },
  SafehouseAllowFire: {
    section: "Safehouses",
    description: "Non-members may start fires inside claimed safehouses.",
  },
  SafehouseAllowLoot: {
    section: "Safehouses",
    description: "Non-members may loot containers inside claimed safehouses.",
  },
  SafehouseAllowRespawn: {
    section: "Safehouses",
    description: "Players may set a claimed safehouse as their respawn point.",
  },
  SafehouseDaySurvivedToClaim: {
    section: "Safehouses",
    description: "In-game days a player must survive before claiming a safehouse.",
    min: 0,
  },
  SafeHouseRemovalTime: {
    section: "Safehouses",
    description: "Hours of inactivity before an unclaimed safehouse is released.",
    min: 0,
  },

  // -- Saving & Backups --------------------------------------------------------------
  SaveWorldEveryMinutes: {
    section: "Saving",
    description: "World autosave interval.",
    min: 1,
  },
  BackupsOnStart: {
    section: "Saving",
    description: "Snapshot the world each time the server boots.",
    requiresRestart: true,
  },
  BackupsOnVersionChange: {
    section: "Saving",
    description: "Snapshot the world when the game build changes.",
  },
  BackupsPeriod: {
    section: "Saving",
    description: "Minutes between periodic world backups.",
    min: 0,
  },
  BackupsCount: {
    section: "Saving",
    description: "Number of rotating backups kept before the oldest is deleted.",
    min: 0,
  },

  // -- Chat & Voice --------------------------------------------------------------
  GlobalChat: { section: "Chat & Voice", description: "Enable the all-players chat channel." },
  VoiceEnable: {
    section: "Chat & Voice",
    description: "Allow in-game proximity voice chat.",
  },
  Voice3D: {
    section: "Chat & Voice",
    description: "Attenuate voice volume by in-game distance and direction.",
  },
};
