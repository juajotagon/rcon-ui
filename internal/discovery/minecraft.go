package discovery

import (
	"context"
	"regexp"
	"strings"
)

// minecraftWriteTemplate is vanilla's gamerule command: `/gamerule <rule>
// <value>` with an unquoted, unwrapped value.
const minecraftWriteTemplate = "gamerule {key} {value}"

// gamerules is the vanilla Java Edition rule set. This is protocol knowledge,
// like a packet layout, not per-game configuration: rcon-ui ships no list of
// supported games, but knowing the fixed vocabulary of one dialect's query
// command is exactly the kind of fact a dialect implementation is allowed to
// hard-code. A rule this server build doesn't have (an older version, a
// modded server) simply errors or replies unrecognisably and is skipped --
// nothing downstream depends on this list being exhaustive or current.
var gamerules = []string{
	"announceAdvancements", "commandBlockOutput", "commandModificationBlockLimit",
	"disableElytraMovementCheck", "disableRaids", "doDaylightCycle", "doEntityDrops",
	"doFireTick", "doImmediateRespawn", "doInsomnia", "doLimitedCrafting", "doMobLoot",
	"doMobSpawning", "doPatrolSpawning", "doTileDrops", "doTraderSpawning",
	"doVinesSpread", "doWardenSpawning", "doWeatherCycle", "drowningDamage",
	"enderPearlsVanishOnDeath", "fallDamage", "fireDamage", "forgiveDeadPlayers",
	"freezeDamage", "globalSoundEvents", "keepInventory", "lavaSourceConversion",
	"logAdminCommands", "maxCommandChainLength", "maxCommandForkCount",
	"maxEntityCramming", "mobExplosionDropDecay", "mobGriefing", "naturalRegeneration",
	"playersNetherPortalCreativeDelay", "playersNetherPortalDefaultDelay",
	"playersSleepingPercentage", "projectilesCanBreakBlocks", "randomTickSpeed",
	"reducedDebugInfo", "sendCommandFeedback", "showDeathMessages", "snowAccumulationHeight",
	"spawnChunkRadius", "spawnRadius", "spectatorsGenerateChunks", "tntExplosionDropDecay",
	"universalAnger", "waterSourceConversion",
}

// minecraftGamerule matches vanilla's echo of a gamerule query, e.g.
// "Gamerule randomTickSpeed is currently set to: 3". The colon before the
// value is optional and "currently" is too, since both have varied across
// releases; only the shape that matters for parsing is required.
var minecraftGamerule = regexp.MustCompile(`(?i)gamerule\s+(\S+)\s+is\s+(?:currently\s+)?set to:?\s*(.+)$`)

func looksLikeMinecraftGamerule(reply string) bool {
	return minecraftGamerule.MatchString(strings.TrimSpace(reply))
}

// discoverGamerules queries every known vanilla rule and keeps the ones the
// server recognises. This is the one place discovery makes many probe calls,
// so it goes through the caller-supplied exec directly rather than any
// history- or event-producing path -- flooding the console with 40 gamerule
// queries would make the console useless for the command a user actually
// typed.
func discoverGamerules(ctx context.Context, exec func(context.Context, string) (string, error)) []Option {
	var out []Option
	for _, rule := range gamerules {
		reply, err := exec(ctx, "gamerule "+rule)
		if err != nil {
			continue
		}
		m := minecraftGamerule.FindStringSubmatch(strings.TrimSpace(reply))
		if m == nil {
			continue
		}
		value := strings.TrimSpace(m[2])
		out = append(out, Option{Key: rule, Value: value, Type: inferType(value)})
	}
	return out
}

// minecraftHelpCommand matches a slash-command token at the start of a help
// line. Vanilla's RCON `help` output is a paginated wall of usage syntax
// ("/gamerule <rule> [<value>]"), not the tidy "name : description" pairs PZ
// produces, so this extracts names only -- there is no reliable description
// or usage field to pull out of it.
var minecraftHelpCommand = regexp.MustCompile(`(?m)^/([A-Za-z][A-Za-z0-9_-]*)`)

// minGenericCommands is the smallest match count treated as a real command
// listing rather than noise. A handful of coincidental "/word" matches in
// garbage text is plausible; dozens in a row is not.
const minGenericCommands = 3

// parseGenericHelp best-effort extracts command names from a help reply whose
// shape isn't known ahead of time -- vanilla Minecraft, or a dialect this
// package has never seen. If too few slash-commands turn up to look like a
// real catalog, it returns nil so the caller ships capabilities.commands =
// false and an empty list instead of a handful of noise entries.
func parseGenericHelp(reply string) []Command {
	seen := map[string]bool{}
	var out []Command
	for _, m := range minecraftHelpCommand.FindAllStringSubmatch(reply, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, Command{Name: name})
	}
	if len(out) < minGenericCommands {
		return nil
	}
	return out
}
