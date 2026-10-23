// Command ascprune reduces the App Store Connect OpenAPI spec to the operations
// listed in operations.txt and every component schema they reference, so the
// client generator only sees what the CLI uses.
//
// Key order of the input is kept (the generator relies on property order) and
// the output is re-indented, so the result is byte-identical for the same input.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
)

const schemaRefPrefix = "#/components/schemas/"

func main() {
	specPath := flag.String("spec", "", "input OpenAPI spec (JSON)")
	opsPath := flag.String("ops", "", "file with one operation ID per line, # comments")
	outPath := flag.String("out", "", "output path for the pruned spec")
	flag.Parse()
	if *specPath == "" || *opsPath == "" || *outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*specPath, *opsPath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, "ascprune:", err)
		os.Exit(1)
	}
}

func run(specPath, opsPath, outPath string) error {
	ops, err := readOperations(opsPath)
	if err != nil {
		return err
	}
	spec, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	pruned, err := prune(spec, ops)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, pruned, "", "  "); err != nil {
		return err
	}
	out.WriteByte('\n')
	return os.WriteFile(outPath, out.Bytes(), 0o644)
}

// readOperations returns the operation IDs in the file. Blank lines and lines
// starting with # are ignored; an ID listed twice is an error.
func readOperations(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ops := map[string]bool{}
	for line := range strings.Lines(string(b)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if ops[line] {
			return nil, fmt.Errorf("%s: operation %s is listed twice", path, line)
		}
		ops[line] = true
	}
	return ops, nil
}

// prune returns the spec with only the listed operations and the schemas they
// reference, transitively. Everything else at the top level is kept as is.
func prune(spec []byte, ops map[string]bool) ([]byte, error) {
	top, err := members(spec)
	if err != nil {
		return nil, err
	}
	var paths, components []member
	for _, m := range top {
		switch m.Key {
		case "paths":
			paths, err = members(m.Value)
		case "components":
			components, err = members(m.Value)
		}
		if err != nil {
			return nil, err
		}
	}

	found := map[string]bool{}
	var keptPaths []member
	for _, p := range paths {
		item, err := members(p.Value)
		if err != nil {
			return nil, err
		}
		var kept []member
		hasOp := false
		for _, m := range item {
			if m.Key == "parameters" {
				kept = append(kept, m)
				continue
			}
			var op struct{ OperationID string }
			if err := json.Unmarshal(m.Value, &op); err != nil {
				return nil, fmt.Errorf("%s %s: %w", m.Key, p.Key, err)
			}
			if !ops[op.OperationID] {
				continue
			}
			if found[op.OperationID] {
				return nil, fmt.Errorf("operation %s is defined twice", op.OperationID)
			}
			found[op.OperationID] = true
			kept = append(kept, m)
			hasOp = true
		}
		if hasOp {
			keptPaths = append(keptPaths, member{p.Key, object(kept)})
		}
	}
	var missing []string
	for id := range ops {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("operations not in spec: %s", strings.Join(missing, ", "))
	}

	var schemas []member
	for _, c := range components {
		if c.Key == "schemas" {
			if schemas, err = members(c.Value); err != nil {
				return nil, err
			}
		}
	}
	keep, err := referencedSchemas(object(keptPaths), schemas)
	if err != nil {
		return nil, err
	}
	var keptSchemas []member
	for _, s := range schemas {
		if keep[s.Key] {
			keptSchemas = append(keptSchemas, s)
		}
	}

	var out []member
	for _, m := range top {
		switch m.Key {
		case "paths":
			m.Value = object(keptPaths)
		case "components":
			var kept []member
			for _, c := range components {
				if c.Key == "schemas" {
					c.Value = object(keptSchemas)
				}
				kept = append(kept, c)
			}
			m.Value = object(kept)
		}
		out = append(out, m)
	}
	return object(out), nil
}

// referencedSchemas returns the names of the schemas referenced from root,
// directly or through other schemas.
func referencedSchemas(root json.RawMessage, schemas []member) (map[string]bool, error) {
	byName := map[string]json.RawMessage{}
	for _, s := range schemas {
		byName[s.Key] = s.Value
	}
	keep := map[string]bool{}
	queue := []json.RawMessage{root}
	for len(queue) > 0 {
		names, err := schemaRefs(queue[0])
		if err != nil {
			return nil, err
		}
		queue = queue[1:]
		for _, name := range names {
			if keep[name] {
				continue
			}
			s, ok := byName[name]
			if !ok {
				return nil, fmt.Errorf("unresolved schema reference %s", name)
			}
			keep[name] = true
			queue = append(queue, s)
		}
	}
	return keep, nil
}

// schemaRefs returns the schema names of all "$ref" values in raw. Only
// schemas are kept, so a reference to anything else is an error.
func schemaRefs(raw json.RawMessage) ([]string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	var names, others []string
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if ref, ok := t["$ref"].(string); ok {
				if name, ok := strings.CutPrefix(ref, schemaRefPrefix); ok {
					names = append(names, name)
				} else {
					others = append(others, ref)
				}
			}
			for _, c := range t {
				walk(c)
			}
		case []any:
			for _, c := range t {
				walk(c)
			}
		}
	}
	walk(v)
	if len(others) > 0 {
		slices.Sort(others)
		return nil, fmt.Errorf("unsupported references (only %s is kept): %s", schemaRefPrefix, strings.Join(slices.Compact(others), ", "))
	}
	return names, nil
}

type member struct {
	Key   string
	Value json.RawMessage
}

// members returns the members of a JSON object in document order.
func members(raw json.RawMessage) ([]member, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("expected JSON object")
	}
	var ms []member
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		ms = append(ms, member{tok.(string), v})
	}
	return ms, nil
}

// object encodes members as a JSON object in the given order.
func object(ms []member) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range ms {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(m.Key)
		b.Write(k)
		b.WriteByte(':')
		b.Write(m.Value)
	}
	b.WriteByte('}')
	return b.Bytes()
}
