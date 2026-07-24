package discovery

import (
	"regexp"
	"strings"
)

// zomboidWriteTemplate is Project Zomboid's changeoption syntax. The value is
// always quoted on the wire regardless of type -- the server accepts
// changeoption PVP "true" for a boolean same as for a string -- so the
// template needs no per-type variant.
const zomboidWriteTemplate = `changeoption {key} "{value}"`

// zomboidOptionLine matches one line of a `showoptions` dump. The leading
// "* " is tolerated as optional because it is cosmetic -- some server builds
// and mods have been seen to omit it -- and blank lines are skipped by the
// caller rather than by the pattern.
var zomboidOptionLine = regexp.MustCompile(`^\*?\s*([A-Za-z][A-Za-z0-9_.]*)\s*=\s*(.*)$`)

// looksLikeZomboidOptions reports whether reply is a `showoptions` dump. The
// header is the strongest signal when present, but is not required: a dump
// with at least one recognisable "Key=value" line is accepted too, so a
// server build that varies the header wording still fingerprints correctly.
func looksLikeZomboidOptions(reply string) bool {
	if strings.Contains(strings.ToLower(reply), "list of server options") {
		return true
	}
	return len(parseZomboidOptions(reply)) > 0
}

func parseZomboidOptions(reply string) []Option {
	var out []Option
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := zomboidOptionLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key, value := m[1], strings.TrimSpace(m[2])
		out = append(out, Option{Key: key, Value: value, Type: inferType(value)})
	}
	return out
}

// zomboidHelpLine matches one entry of a `help` reply: "* name : description".
var zomboidHelpLine = regexp.MustCompile(`^\*\s*(\S+)\s*:\s*(.*)$`)

// zomboidUsage pulls the "Use: ..." fragment out of a command's description,
// stopping before a trailing "Example:" clause when one follows.
var zomboidUsage = regexp.MustCompile(`(?i)Use:\s*(.*?)(?:\s+Example:|$)`)

// parseZomboidHelp turns a `help` reply into a command catalog. PZ's help
// text is consistently "* name : description", so this is a straight
// line-by-line parse rather than the best-effort guessing a less structured
// dialect needs.
func parseZomboidHelp(reply string) []Command {
	var out []Command
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		m := zomboidHelpLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, desc := m[1], strings.TrimSpace(m[2])

		usage := ""
		if um := zomboidUsage.FindStringSubmatch(desc); um != nil {
			usage = strings.TrimSpace(um[1])
		}
		out = append(out, Command{Name: name, Description: desc, Usage: usage})
	}
	return out
}
