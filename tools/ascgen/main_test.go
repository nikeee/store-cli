package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The generator rejects what it cannot map instead of panicking or
// generating wrong code.
func TestOperationRejectsUnsupported(t *testing.T) {
	tests := []struct{ name, op, want string }{
		{"ok", `{"operationId":"x","parameters":[{"name":"filter[a]","in":"query","style":"form","explode":false,"schema":{"type":"array"}}],
			"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/R"}}}}}}`, ""},
		{"parameter ref", `{"operationId":"x","parameters":[{"$ref":"#/components/parameters/p"}]}`, "parameter $ref #/components/parameters/p is not supported"},
		{"no schema", `{"operationId":"x","parameters":[{"name":"q","in":"query","content":{"application/json":{}}}]}`, "parameter q has no inline schema"},
		{"exploded array", `{"operationId":"x","parameters":[{"name":"ids","in":"query","schema":{"type":"array"}}]}`,
			"query parameter ids: only style form with explode false (comma separated) is supported"},
		{"response ref", `{"operationId":"x","responses":{"200":{"$ref":"#/components/responses/R"}}}`, "response 200: $ref #/components/responses/R is not supported"},
		{"media without schema", `{"operationId":"x","responses":{"200":{"content":{"application/json":{}}}}}`, "response 200: only a single application/json $ref is supported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw rawOperation
			if err := json.Unmarshal([]byte(tt.op), &raw); err != nil {
				t.Fatal(err)
			}
			s := &spec{}
			s.Components.Schemas = map[string]*schema{"R": {Type: "object"}}
			g := &gen{spec: s, memo: map[*schema]string{}, idents: map[string]string{}, resourceTypes: map[string]bool{}}
			_, err := g.operation("/v1/x", "GET", raw, raw.Parameters)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}
}
