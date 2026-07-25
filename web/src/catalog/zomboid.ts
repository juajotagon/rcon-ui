import type { GameTemplate } from "./types";

/** Curated servertest.ini keys for Project Zomboid, plus a representative
 * admin command set.
 *
 * Deliberately not exhaustive -- Zomboid's option dump runs past a hundred
 * keys across mod-added entries, and any list here will lag the game. Keys
 * missing from this table (Mods, Map, ServerPlayerID, ...) still show up via
 * `showoptions` on a live server, just under "Other" with no description --
 * see enrich/index.ts for how a missing entry degrades, never hides.
 *
 * This is the single source of truth for Zomboid enrichment: `metaFor` in
 * enrich/index.ts reads straight from `options` below, so the live Settings
 * panel and this catalog can never drift apart.
 */
export const zomboidTemplate: GameTemplate = {
  id: "zomboid",
  name: "Project Zomboid",
  defaultPort: 27015,
  capabilities: { options: true, optionWrite: true, commands: true },
  writeTemplate: 'changeoption {key} "{value}"',
  caveats: [
    "The options dump (showoptions) only lists keys the running build recognizes -- mod-added settings and newer keys can be missing from a given server's list.",
    "Some options only take effect after the server restarts (see the restart badge below); a live changeoption still succeeds but does nothing until then.",
  ],
  options: [
    // -- Server --------------------------------------------------------------
    {
      key: "Public",
      example: "true",
      section: "Server",
      description: "List this server in the public in-game browser.",
    },
    {
      key: "PublicName",
      example: "My Zomboid Server",
      section: "Server",
      description: "Name shown in the server browser.",
    },
    {
      key: "PublicDescription",
      example: "Whitelisted PvE server, mods: Brita's, ORGM",
      section: "Server",
      description: "Short blurb shown in the server browser listing.",
    },
    {
      key: "MaxPlayers",
      example: "32",
      section: "Server",
      description: "Player slots. Read at startup; takes effect after the next restart.",
      min: 1,
      max: 100,
      requiresRestart: true,
    },
    {
      key: "Open",
      example: "true",
      section: "Server",
      description: "Allow joining without whitelist approval.",
    },
    {
      key: "PingLimit",
      example: "400",
      section: "Server",
      description: "Kick players whose ping stays above this (ms).",
      min: 0,
      max: 999,
    },
    {
      key: "AutoCreateUserInWhiteList",
      example: "false",
      section: "Server",
      description: "Automatically whitelist a username the first time it logs in.",
    },
    {
      key: "DisplayUserName",
      example: "true",
      section: "Server",
      description: "Show player names above their characters.",
    },
    {
      key: "ServerWelcomeMessage",
      example: "Welcome! Read the rules in #info.",
      section: "Server",
      description: "Message shown in chat when a player joins.",
    },
    {
      key: "LoginQueueEnabled",
      example: "false",
      section: "Server",
      description: "Queue joins when the server is full instead of rejecting them.",
    },
    {
      key: "LoginQueueConnectTimeout",
      example: "60",
      section: "Server",
      description: "Seconds a queued connection is held before it's dropped.",
      min: 0,
    },

    // -- Gameplay --------------------------------------------------------------
    {
      key: "PVP",
      example: "true",
      section: "Gameplay",
      description: "Player-versus-player damage anywhere outside safe zones.",
    },
    {
      key: "PVPMeleeDamageModifier",
      example: "30",
      section: "Gameplay",
      description: "Melee damage multiplier (%) applied to PVP hits.",
      min: 0,
      max: 500,
    },
    {
      key: "PVPFirearmDamageModifier",
      example: "50",
      section: "Gameplay",
      description: "Firearm damage multiplier (%) applied to PVP hits.",
      min: 0,
      max: 500,
    },
    {
      key: "PauseEmpty",
      example: "true",
      section: "Gameplay",
      description: "Freeze world time while nobody is online.",
    },
    {
      key: "AnnounceDeath",
      example: "true",
      section: "Gameplay",
      description: "Broadcast a chat line when a player dies.",
    },
    {
      key: "SpeedLimit",
      example: "70",
      section: "Gameplay",
      description: "Vehicle speed cap.",
      min: 0,
      max: 200,
    },
    {
      key: "KnockedDownAllowed",
      example: "true",
      section: "Gameplay",
      description: "Players can be knocked down in melee.",
    },
    {
      key: "SleepAllowed",
      example: "true",
      section: "Gameplay",
      description: "Players can sleep in beds to pass time.",
    },
    {
      key: "SleepNeeded",
      example: "false",
      section: "Gameplay",
      description: "Players suffer stat penalties if they don't sleep.",
    },
    {
      key: "PlayerRespawnWithSelf",
      example: "false",
      section: "Gameplay",
      description: "New character spawns with the same inventory as the last death.",
    },
    {
      key: "PlayerRespawnWithOther",
      example: "false",
      section: "Gameplay",
      description: "New character can loot their own corpse's items from another player.",
    },

    // -- Safehouses --------------------------------------------------------------
    {
      key: "SafehouseAllowTrepass",
      example: "true",
      section: "Safehouses",
      description: "Non-members may enter claimed safehouses.",
    },
    {
      key: "SafehouseAllowFire",
      example: "false",
      section: "Safehouses",
      description: "Non-members may start fires inside claimed safehouses.",
    },
    {
      key: "SafehouseAllowLoot",
      example: "false",
      section: "Safehouses",
      description: "Non-members may loot containers inside claimed safehouses.",
    },
    {
      key: "SafehouseAllowRespawn",
      example: "false",
      section: "Safehouses",
      description: "Players may set a claimed safehouse as their respawn point.",
    },
    {
      key: "SafehouseDaySurvivedToClaim",
      example: "0",
      section: "Safehouses",
      description: "In-game days a player must survive before claiming a safehouse.",
      min: 0,
    },
    {
      key: "SafeHouseRemovalTime",
      example: "144",
      section: "Safehouses",
      description: "Hours of inactivity before an unclaimed safehouse is released.",
      min: 0,
    },

    // -- Saving & Backups --------------------------------------------------------------
    {
      key: "SaveWorldEveryMinutes",
      example: "15",
      section: "Saving",
      description: "World autosave interval.",
      min: 1,
    },
    {
      key: "BackupsOnStart",
      example: "true",
      section: "Saving",
      description: "Snapshot the world each time the server boots.",
      requiresRestart: true,
    },
    {
      key: "BackupsOnVersionChange",
      example: "true",
      section: "Saving",
      description: "Snapshot the world when the game build changes.",
    },
    {
      key: "BackupsPeriod",
      example: "30",
      section: "Saving",
      description: "Minutes between periodic world backups.",
      min: 0,
    },
    {
      key: "BackupsCount",
      example: "5",
      section: "Saving",
      description: "Number of rotating backups kept before the oldest is deleted.",
      min: 0,
    },

    // -- Chat & Voice --------------------------------------------------------------
    {
      key: "GlobalChat",
      example: "true",
      section: "Chat & Voice",
      description: "Enable the all-players chat channel.",
    },
    {
      key: "VoiceEnable",
      example: "true",
      section: "Chat & Voice",
      description: "Allow in-game proximity voice chat.",
    },
    {
      key: "Voice3D",
      example: "true",
      section: "Chat & Voice",
      description: "Attenuate voice volume by in-game distance and direction.",
    },
  ],
  commands: [
    { name: "players", description: "List connected players." },
    {
      name: "kickuser",
      description: "Disconnect a player from the server.",
      usage: 'kickuser "playername"',
    },
    {
      name: "banuser",
      description: "Ban a player by username.",
      usage: 'banuser "playername" [-ip] [-r reason]',
    },
    {
      name: "unbanuser",
      description: "Remove a username from the ban list.",
      usage: 'unbanuser "playername"',
    },
    {
      name: "grantadmin",
      description: "Grant admin rights to a player.",
      usage: 'grantadmin "playername"',
    },
    {
      name: "removeadmin",
      description: "Revoke admin rights from a player.",
      usage: 'removeadmin "playername"',
    },
    {
      name: "servermsg",
      description: "Broadcast a message to all players.",
      usage: 'servermsg "message text"',
    },
    { name: "save", description: "Force an immediate world save." },
    { name: "quit", description: "Save and shut down the server." },
    {
      name: "teleport",
      description: "Teleport one player to another.",
      usage: 'teleport "player" "toPlayer"',
    },
  ],
};
