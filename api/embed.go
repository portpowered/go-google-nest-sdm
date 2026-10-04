// Package api embeds generated, offline JSON Schema runtime contracts.
package api

import "embed"

// RuntimeSchemas holds Draft 7 projections of canonical contract components.
//
//go:embed contracts/*.yaml
var RuntimeSchemas embed.FS
