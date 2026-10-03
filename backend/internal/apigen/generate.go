// Package apigen holds the server interface and models generated from api/openapi.yaml.
// Regenerate with `go generate ./...` after changing the spec.
package apigen

//go:generate go tool oapi-codegen -config ../../oapi-codegen.yaml -o api.gen.go ../../../api/openapi.yaml
