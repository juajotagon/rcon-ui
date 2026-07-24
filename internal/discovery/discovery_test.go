package discovery_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juajotagon/rcon-ui/internal/discovery"
)

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return string(data)
}

func optionsByKey(opts []discovery.Option) map[string]discovery.Option {
	out := make(map[string]discovery.Option, len(opts))
	for _, o := range opts {
		out[o.Key] = o
	}
	return out
}

func commandsByName(cmds []discovery.Command) map[string]discovery.Command {
	out := make(map[string]discovery.Command, len(cmds))
	for _, c := range cmds {
		out[c.Name] = c
	}
	return out
}

// TestDiscoverZomboid feeds Discover a real captured showoptions dump (16
// option lines, including a missing "* " prefix and a blank line to prove
// both are tolerated) and a real captured help transcript (60 commands),
// exactly as recorded from a live server in the TLS smoke test. Two of the
// options are credential-shaped and must never survive into the Report.
func TestDiscoverZomboid(t *testing.T) {
	showoptions := readTestdata(t, "pz_showoptions.txt")
	help := readTestdata(t, "pz_help.txt")

	exec := func(_ context.Context, cmd string) (string, error) {
		switch cmd {
		case "showoptions":
			return showoptions, nil
		case "help":
			return help, nil
		default:
			return "", errors.New("unrecognised command: " + cmd)
		}
	}

	report := discovery.Discover(context.Background(), exec)

	if report.Fingerprint != discovery.FingerprintZomboid {
		t.Fatalf("fingerprint = %q, want %q", report.Fingerprint, discovery.FingerprintZomboid)
	}
	if report.Label != "Project Zomboid" {
		t.Errorf("label = %q, want %q", report.Label, "Project Zomboid")
	}
	if report.WriteTemplate != `changeoption {key} "{value}"` {
		t.Errorf("writeTemplate = %q", report.WriteTemplate)
	}
	wantCaps := discovery.Capabilities{Commands: true, Options: true, OptionWrite: true}
	if report.Capabilities != wantCaps {
		t.Errorf("capabilities = %+v, want %+v", report.Capabilities, wantCaps)
	}

	// 16 lines in the fixture, minus the 2 credential-shaped keys.
	const wantOptions = 14
	if len(report.Options) != wantOptions {
		t.Fatalf("got %d options, want %d: %+v", len(report.Options), wantOptions, report.Options)
	}
	opts := optionsByKey(report.Options)
	for _, secret := range []string{"Password", "AdminPassword"} {
		if _, ok := opts[secret]; ok {
			t.Errorf("credential-shaped option %q leaked into the report", secret)
		}
	}

	for key, want := range map[string]discovery.Option{
		"PVP":                       {Key: "PVP", Value: "true", Type: discovery.TypeBoolean},
		"AutoCreateUserInWhiteList": {Key: "AutoCreateUserInWhiteList", Value: "false", Type: discovery.TypeBoolean},
		"MaxPlayers":                {Key: "MaxPlayers", Value: "32", Type: discovery.TypeInteger},
		"SafetyToggleTimer":         {Key: "SafetyToggleTimer", Value: "2", Type: discovery.TypeInteger},
		"MinutesPerPage":            {Key: "MinutesPerPage", Value: "1.0", Type: discovery.TypeNumber},
		"Map":                       {Key: "Map", Value: "Muldraugh, KY", Type: discovery.TypeString},
		"SpawnPoint":                {Key: "SpawnPoint", Value: "0,0,0", Type: discovery.TypeString},
	} {
		got, ok := opts[key]
		if !ok {
			t.Errorf("missing option %q", key)
			continue
		}
		if got != want {
			t.Errorf("option %q = %+v, want %+v", key, got, want)
		}
	}

	const wantCommands = 60
	if len(report.Commands) != wantCommands {
		t.Fatalf("got %d commands, want %d", len(report.Commands), wantCommands)
	}
	cmds := commandsByName(report.Commands)

	additem, ok := cmds["additem"]
	if !ok {
		t.Fatal("missing additem command")
	}
	if want := `/additem "username" "module.item" count.`; additem.Usage != want {
		t.Errorf("additem usage = %q, want %q", additem.Usage, want)
	}
	if !strings.Contains(additem.Description, "Give an item to a player") {
		t.Errorf("additem description = %q", additem.Description)
	}

	help1, ok := cmds["help"]
	if !ok {
		t.Fatal("missing help command")
	}
	if help1.Usage != "" {
		t.Errorf("help usage = %q, want empty (no Use: fragment in the source line)", help1.Usage)
	}
}

// TestDiscoverMinecraft simulates a vanilla server: showoptions is an
// unrecognised command (as it is on every non-Zomboid server), the gamerule
// probe fingerprints it, and the fixture exercises both ways a rule in
// rcon-ui's static vanilla list can legitimately fail to yield an option --
// doVinesSpread answers but not recognisably, freezeDamage isn't known to
// this fake server at all -- proving neither aborts the sweep.
func TestDiscoverMinecraft(t *testing.T) {
	help := readTestdata(t, "minecraft_help.txt")

	ok := map[string]string{}
	garbled := map[string]string{}
	for _, line := range strings.Split(readTestdata(t, "minecraft_gamerules.txt"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			t.Fatalf("malformed fixture line: %q", line)
		}
		rule, status, payload := parts[0], parts[1], parts[2]
		switch status {
		case "ok":
			ok[rule] = payload
		case "garbled":
			garbled[rule] = payload
		case "missing":
			// deliberately absent from both maps
		default:
			t.Fatalf("unknown fixture status %q", status)
		}
	}

	exec := func(_ context.Context, cmd string) (string, error) {
		switch {
		case cmd == "showoptions":
			return "", errors.New("unknown command")
		case cmd == "help":
			return help, nil
		case strings.HasPrefix(cmd, "gamerule "):
			rule := strings.TrimPrefix(cmd, "gamerule ")
			if v, found := ok[rule]; found {
				return "Gamerule " + rule + " is currently set to: " + v, nil
			}
			if v, found := garbled[rule]; found {
				return v, nil
			}
			return "", errors.New("no such rule")
		default:
			return "", errors.New("unrecognised command: " + cmd)
		}
	}

	report := discovery.Discover(context.Background(), exec)

	if report.Fingerprint != discovery.FingerprintMinecraft {
		t.Fatalf("fingerprint = %q, want %q", report.Fingerprint, discovery.FingerprintMinecraft)
	}
	if report.Label != "Minecraft" {
		t.Errorf("label = %q, want %q", report.Label, "Minecraft")
	}
	if report.WriteTemplate != "gamerule {key} {value}" {
		t.Errorf("writeTemplate = %q", report.WriteTemplate)
	}
	wantCaps := discovery.Capabilities{Commands: true, Options: true, OptionWrite: true}
	if report.Capabilities != wantCaps {
		t.Errorf("capabilities = %+v, want %+v", report.Capabilities, wantCaps)
	}

	if len(report.Options) != len(ok) {
		t.Fatalf("got %d options, want %d (one per 'ok' fixture rule): %+v", len(report.Options), len(ok), report.Options)
	}
	opts := optionsByKey(report.Options)
	for _, unresolved := range []string{"doVinesSpread", "freezeDamage"} {
		if _, found := opts[unresolved]; found {
			t.Errorf("rule %q should have been skipped, not reported", unresolved)
		}
	}
	for key, want := range map[string]discovery.Option{
		"mobGriefing":               {Key: "mobGriefing", Value: "false", Type: discovery.TypeBoolean},
		"randomTickSpeed":           {Key: "randomTickSpeed", Value: "3", Type: discovery.TypeInteger},
		"maxCommandChainLength":     {Key: "maxCommandChainLength", Value: "65536", Type: discovery.TypeInteger},
		"playersSleepingPercentage": {Key: "playersSleepingPercentage", Value: "100", Type: discovery.TypeInteger},
		"doDaylightCycle":           {Key: "doDaylightCycle", Value: "true", Type: discovery.TypeBoolean},
	} {
		got, found := opts[key]
		if !found {
			t.Errorf("missing option %q", key)
			continue
		}
		if got != want {
			t.Errorf("option %q = %+v, want %+v", key, got, want)
		}
	}

	wantCommandNames := []string{
		"advancement", "attribute", "ban", "ban-ip", "banlist", "bossbar",
		"clear", "clone", "data", "datapack", "debug", "defaultgamemode",
		"deop", "difficulty", "effect", "enchant", "execute", "experience",
		"fill", "forceload", "function", "gamemode", "gamerule", "give",
	}
	if len(report.Commands) != len(wantCommandNames) {
		t.Fatalf("got %d commands, want %d: %+v", len(report.Commands), len(wantCommandNames), report.Commands)
	}
	cmds := commandsByName(report.Commands)
	for _, name := range wantCommandNames {
		if _, found := cmds[name]; !found {
			t.Errorf("missing best-effort command %q", name)
		}
	}
}

// TestDiscoverUnknownFromGarbage proves that a reply matching none of the
// known dialects lands cleanly on the unknown fingerprint with no error and
// no noise -- not a crash, and not a handful of coincidental regex matches
// mistaken for real options or commands.
func TestDiscoverUnknownFromGarbage(t *testing.T) {
	garbage := readTestdata(t, "garbage.txt")
	exec := func(_ context.Context, _ string) (string, error) {
		return garbage, nil
	}

	report := discovery.Discover(context.Background(), exec)

	if report.Fingerprint != discovery.FingerprintUnknown {
		t.Fatalf("fingerprint = %q, want %q", report.Fingerprint, discovery.FingerprintUnknown)
	}
	if report.Label != "Unidentified" {
		t.Errorf("label = %q, want %q", report.Label, "Unidentified")
	}
	if report.WriteTemplate != "" {
		t.Errorf("writeTemplate = %q, want empty", report.WriteTemplate)
	}
	if report.Capabilities != (discovery.Capabilities{}) {
		t.Errorf("capabilities = %+v, want all false", report.Capabilities)
	}
	if report.Options == nil || len(report.Options) != 0 {
		t.Errorf("options = %#v, want non-nil empty slice", report.Options)
	}
	if report.Commands == nil || len(report.Commands) != 0 {
		t.Errorf("commands = %#v, want non-nil empty slice", report.Commands)
	}
}

// TestDiscoverEveryProbeFailingStillReturnsAReport is the degrade-not-abort
// guarantee taken to its extreme: every single exec call errors (a
// disconnected or hung session), and Discover must still hand back a usable,
// non-nil Report rather than panicking or blocking past its budget.
func TestDiscoverEveryProbeFailingStillReturnsAReport(t *testing.T) {
	exec := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("not connected")
	}

	report := discovery.Discover(context.Background(), exec)

	if report.Fingerprint != discovery.FingerprintUnknown {
		t.Errorf("fingerprint = %q, want %q", report.Fingerprint, discovery.FingerprintUnknown)
	}
	if report.Options == nil || report.Commands == nil {
		t.Errorf("slices must stay non-nil even when every probe fails: options=%#v commands=%#v", report.Options, report.Commands)
	}
}

// TestReportSlicesSerialiseAsEmptyArrays pins the JSON contract the frontend
// is being built against: a Report with no discovered commands or options
// must marshal those fields as [], never null.
func TestReportSlicesSerialiseAsEmptyArrays(t *testing.T) {
	exec := func(_ context.Context, _ string) (string, error) { return "", errors.New("nope") }
	report := discovery.Discover(context.Background(), exec)

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)
	if strings.Contains(got, `"commands":null`) {
		t.Errorf("commands serialised as null: %s", got)
	}
	if strings.Contains(got, `"options":null`) {
		t.Errorf("options serialised as null: %s", got)
	}
	if !strings.Contains(got, `"commands":[]`) || !strings.Contains(got, `"options":[]`) {
		t.Errorf("expected empty-array slices, got: %s", got)
	}
}
