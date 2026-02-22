package agent

import "embed"

//go:embed agents/*.md
var EmbeddedAgents embed.FS

//go:embed opencode.json
var EmbeddedOpencodeJSON []byte
