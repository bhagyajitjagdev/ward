// Package migrations embeds the goose SQL migrations so they ship inside the
// single Ward binary.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
