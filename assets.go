// Package agentconfig embeds the vetted content agtk installs, so the binary needs nothing
// else on the target machine. Personal profiles are not embedded; they are read from disk.
package agentconfig

import "embed"

//go:embed catalog base all:packs enforcement/codex-rules skills/baseline.json
var Assets embed.FS
