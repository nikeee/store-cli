// Package asc is the App Store Connect API client generated from
// api/apple/openapi.oas.json, restricted to the operations in
// api/apple/operations.txt. Do not edit the *.gen.go files; change the
// operations list or tools/ascgen and run go generate. How to call it:
// docs/decisions/0001-apple-codegen.md.
package asc

//go:generate go run github.com/nikeee/store-cli/tools/ascprune -spec ../../../api/apple/openapi.oas.json -ops ../../../api/apple/operations.txt -out ../../../api/apple/openapi.pruned.json
//go:generate go run github.com/nikeee/store-cli/tools/ascgen -spec ../../../api/apple/openapi.pruned.json -out .
