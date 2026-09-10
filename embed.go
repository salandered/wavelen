package wavelen

import "embed"

// MigrationsFS is the SQL schema.
// Embedding so an image carries both the app code and the schema it expects
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS

// APISpec is the OpenAPI spec. Used in tests.
//
//go:embed api/api.yaml
var APISpec []byte

// WebFS is the static UI.
// Embedding so an image also carries the UI its API serves.
//
//go:embed web/index.html web/app.js web/images/logo-mini.png web/images/logo.png
var WebFS embed.FS
