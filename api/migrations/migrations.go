// Package migrations embeds the goose SQL migrations so the server binary
// can apply them at startup. sqlc also reads these files as the schema.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
