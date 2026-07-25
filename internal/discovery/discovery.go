package discovery

import (
	"context"
	"time"
)

// budget bounds an entire discovery run. Individual probes degrade (skip)
// on failure rather than abort, so this is a ceiling on total wall-clock
// time -- roughly a dialect's worth of showoptions/gamerule/help round
// trips plus the ~40-query vanilla gamerule sweep -- not a per-command
// timeout.
const budget = 30 * time.Second

// Discover interrogates a live connection and returns a Report describing
// what it found. exec runs one command and returns its raw reply; it is a
// plain function rather than an rcon.Client so this package stays testable
// without a real session and reusable by any dialect that can produce one.
//
// Discover never returns an error. A probe that fails, times out, or comes
// back unrecognisable degrades that part of the Report rather than aborting
// the whole run, because a partial Report -- known commands but no options,
// say -- is more useful to the frontend than none at all.
func Discover(ctx context.Context, exec func(context.Context, string) (string, error)) Report {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	report := Report{
		At:       time.Now().UTC(),
		Commands: []Command{},
		Options:  []Option{},
	}

	if reply, err := exec(ctx, "showoptions"); err == nil && looksLikeZomboidOptions(reply) {
		report.Fingerprint = FingerprintZomboid
		report.Options = parseZomboidOptions(reply)
		report.WriteTemplate = zomboidWriteTemplate
		report.Capabilities.Options = true
		report.Capabilities.OptionWrite = true
	} else if reply, err := exec(ctx, "gamerule randomTickSpeed"); err == nil && looksLikeMinecraftGamerule(reply) {
		report.Fingerprint = FingerprintMinecraft
		report.Options = discoverGamerules(ctx, exec)
		report.WriteTemplate = minecraftWriteTemplate
		report.Capabilities.Options = true
		report.Capabilities.OptionWrite = true
	} else {
		report.Fingerprint = FingerprintUnknown
	}
	report.Label = labelFor(report.Fingerprint)
	report.Options = filterCredentials(report.Options)
	if report.Options == nil {
		report.Options = []Option{}
	}

	if reply, err := exec(ctx, "help"); err == nil {
		var cmds []Command
		if report.Fingerprint == FingerprintZomboid {
			cmds = parseZomboidHelp(reply)
		} else {
			cmds = parseGenericHelp(reply)
		}
		if len(cmds) > 0 {
			report.Commands = cmds
			report.Capabilities.Commands = true
		}
	}

	return report
}
