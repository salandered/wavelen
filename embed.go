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

// AttributionFS carries the third party license files. Not used anywhere.
//
//go:embed attribution
var AttributionFS embed.FS

// WebFS is the static UI.
// Embedding so an image also carries the UI its API serves.
//
//go:embed web/index.html web/style.css web/app.js web/lib.js
//go:embed web/images web/fonts
var WebFS embed.FS
