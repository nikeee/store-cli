package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOperationsRejectsDuplicates(t *testing.T) {
	p := filepath.Join(t.TempDir(), "operations.txt")
	if err := os.WriteFile(p, []byte("# c\napps_getCollection\nbuilds_getCollection\napps_getCollection\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readOperations(p); err == nil || !strings.Contains(err.Error(), "apps_getCollection is listed twice") {
		t.Errorf("err = %v", err)
	}
}

// Only schema references are kept, so any other reference would dangle.
func TestSchemaRefsRejectsOtherRefs(t *testing.T) {
	if _, err := schemaRefs([]byte(`{"a":{"$ref":"#/components/schemas/A"},"b":[{"$ref":"#/components/responses/B"}]}`)); err == nil ||
		!strings.Contains(err.Error(), "#/components/responses/B") {
		t.Errorf("err = %v", err)
	}
	names, err := schemaRefs([]byte(`{"a":{"$ref":"#/components/schemas/A"}}`))
	if err != nil || len(names) != 1 || names[0] != "A" {
		t.Errorf("names = %v, err = %v", names, err)
	}
}
