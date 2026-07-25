package discovery

import (
	"regexp"
	"strconv"
	"strings"
)

// credentialKey matches option keys that plausibly hold an RCON credential or
// other secret rather than a gameplay setting. Case-insensitive because
// dialects are inconsistent about casing (Password, ADMINPASSWORD, authToken).
var credentialKey = regexp.MustCompile(`(?i)password|secret|token`)

// filterCredentials drops any option whose key looks like it holds a secret,
// so a server's RCON password (which showoptions echoes back in cleartext on
// Project Zomboid) can never round-trip through the UI.
func filterCredentials(opts []Option) []Option {
	out := make([]Option, 0, len(opts))
	for _, o := range opts {
		if credentialKey.MatchString(o.Key) {
			continue
		}
		out = append(out, o)
	}
	return out
}

// inferType classifies a raw option value using only its shape: no schema
// exists to consult, so "true"/"false" reads as boolean, a bare integer
// literal as integer, anything else parseable as a float as number, and
// everything else as string.
func inferType(value string) string {
	switch strings.ToLower(value) {
	case "true", "false":
		return TypeBoolean
	}
	if _, err := strconv.Atoi(value); err == nil {
		return TypeInteger
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return TypeNumber
	}
	return TypeString
}
