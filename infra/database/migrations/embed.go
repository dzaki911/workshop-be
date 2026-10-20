// Package migrations menyematkan file SQL migrasi ke dalam binary.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
