// Package migrations exposes the SQL migration files as an embed.FS for
// goose. The migrations themselves are sibling .sql files in this package.
package migrations

import "embed"

// FS is the goose migration filesystem.
//
//go:embed *.sql
var FS embed.FS
