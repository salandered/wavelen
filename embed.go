package wavelen

import "embed"

// MigrationsFS is the SQL schema.
// Embedding so an image carries the schema its code expects (atomic deploy)
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS

// APISpec is the OpenAPI spec. Used in tests.
//
//go:embed api/api.yaml
var APISpec []byte
