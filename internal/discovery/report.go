// Package discovery interrogates a live RCON session to work out which game is
// on the other end and what it can do, without any per-game configuration
// baked into rcon-ui itself.
//
// There is deliberately no table mapping games to features: the daemon has no
// list of "supported" games to fall out of date. Instead it sends a short,
// fixed probe sequence any Source RCON server can answer (or fail to answer,
// which is itself informative) and builds a Report from whatever comes back.
// A dialect this package has never heard of still gets a usable, if empty,
// Report rather than an error.
package discovery

import "time"

// Fingerprint values identify the game a Report was built for. "unknown"
// means the probes ran but nothing recognisable came back -- not that
// discovery failed, since a Report is always produced.
const (
	FingerprintZomboid   = "zomboid"
	FingerprintMinecraft = "minecraft"
	FingerprintUnknown   = "unknown"
)

// labels are the human-facing names for each fingerprint, shown in the UI
// instead of the machine-readable value.
var labels = map[string]string{
	FingerprintZomboid:   "Project Zomboid",
	FingerprintMinecraft: "Minecraft",
	FingerprintUnknown:   "Unidentified",
}

// Capabilities says which parts of a Report a caller can rely on. A false
// here means the corresponding slice or template is intentionally empty, not
// that the probe timed out silently.
type Capabilities struct {
	Commands    bool `json:"commands"`
	Options     bool `json:"options"`
	OptionWrite bool `json:"optionWrite"`
}

// Command is one entry from the server's help catalog.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Usage       string `json:"usage"`
}

// Option is one server setting discovered by probing, with its type inferred
// from the value's shape rather than from any schema -- there is no schema.
type Option struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// Option.Type values.
const (
	TypeBoolean = "boolean"
	TypeInteger = "integer"
	TypeNumber  = "number"
	TypeString  = "string"
)

// Report is what one discovery run produces. The frontend builds its entire
// per-server UI from this and nothing else, so every slice is non-nil: a
// missing capability must serialise as [], never null.
type Report struct {
	Fingerprint   string       `json:"fingerprint"`
	Label         string       `json:"label"`
	At            time.Time    `json:"at"`
	Capabilities  Capabilities `json:"capabilities"`
	WriteTemplate string       `json:"writeTemplate"`
	Commands      []Command    `json:"commands"`
	Options       []Option     `json:"options"`
}

// labelFor returns the human name for a fingerprint, falling back to the
// Unidentified label for anything this package doesn't recognise -- so a
// future fingerprint added here can never leave Label blank.
func labelFor(fingerprint string) string {
	if l, ok := labels[fingerprint]; ok {
		return l
	}
	return labels[FingerprintUnknown]
}
