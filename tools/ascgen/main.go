// Command ascgen generates the App Store Connect client in internal/apple/asc
// from the pruned OpenAPI spec (see tools/ascprune). The mapping rules are
// documented in docs/decisions/0001-apple-codegen.md.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

//go:embed *.tmpl
var templates embed.FS

const schemaRefPrefix = "#/components/schemas/"

func main() {
	specPath := flag.String("spec", "", "pruned OpenAPI spec (JSON)")
	outDir := flag.String("out", "", "output directory of the generated package")
	flag.Parse()
	if *specPath == "" || *outDir == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*specPath, *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "ascgen:", err)
		os.Exit(1)
	}
}

func run(specPath, outDir string) error {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	var s spec
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("parse %s: %w", specPath, err)
	}
	g := &gen{spec: &s, memo: map[*schema]string{}, idents: map[string]string{}, resourceTypes: map[string]bool{}}
	if err := g.types(); err != nil {
		return err
	}
	ops, err := g.operations()
	if err != nil {
		return err
	}
	if len(s.Servers) == 0 {
		return fmt.Errorf("spec has no servers")
	}
	data := map[string]any{
		"BaseURL":     strings.TrimSuffix(s.Servers[0].URL, "/"),
		"Decls":       g.decls,
		"TypeImports": g.typeImports(),
		"Ops":         ops,
	}
	tmpl, err := template.ParseFS(templates, "*.tmpl")
	if err != nil {
		return err
	}
	for _, name := range []string{"client", "types", "operations"} {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name+".go.tmpl", data); err != nil {
			return err
		}
		src, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("format %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(outDir, name+".gen.go"), src, 0o644); err != nil {
			return err
		}
	}
	return nil
}

type spec struct {
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]*schema `json:"schemas"`
	} `json:"components"`
}

type schema struct {
	Ref                  string         `json:"$ref"`
	Type                 string         `json:"type"`
	Format               string         `json:"format"`
	Enum                 []string       `json:"enum"`
	Nullable             bool           `json:"nullable"`
	Properties           properties     `json:"properties"`
	Required             []string       `json:"required"`
	Items                *schema        `json:"items"`
	AdditionalProperties *schema        `json:"additionalProperties"`
	OneOf                []*schema      `json:"oneOf"`
	AllOf                []*schema      `json:"allOf"`
	Discriminator        *discriminator `json:"discriminator"`
}

type discriminator struct {
	PropertyName string `json:"propertyName"`
}

type property struct {
	Name   string
	Schema *schema
}

// properties keeps the document order, which becomes the struct field order
// and so the JSON field order of requests.
type properties []property

func (p *properties) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return fmt.Errorf("properties: expected object")
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		var s schema
		if err := dec.Decode(&s); err != nil {
			return err
		}
		*p = append(*p, property{tok.(string), &s})
	}
	return nil
}

// decl is one generated type declaration.
type decl struct {
	Name   string
	Source string // spec location, for the doc comment
	Kind   string // "struct", "enum" or "def"
	Fields []field
	Values []enumValue
	Type   string // underlying type of a "def"
}

type field struct {
	Name, Type, JSONName string
	Omit                 string // "", "omitempty" or "omitzero"
}

type enumValue struct{ Const, Value string }

type gen struct {
	spec          *spec
	decls         []*decl
	memo          map[*schema]string // inline schema -> declared type name
	idents        map[string]string  // package-level identifier -> spec location
	resourceTypes map[string]bool    // values of the shared ResourceType enum
}

// typeImports returns the packages the declared types use.
func (g *gen) typeImports() []string {
	uses := map[string]bool{}
	for _, d := range g.decls {
		types := []string{d.Type}
		for _, f := range d.Fields {
			types = append(types, f.Type)
		}
		for _, t := range types {
			uses["encoding/json"] = uses["encoding/json"] || strings.Contains(t, "json.")
			uses["time"] = uses["time"] || strings.Contains(t, "time.")
		}
	}
	var imports []string
	for _, pkg := range sortedKeys(uses) {
		if uses[pkg] {
			imports = append(imports, pkg)
		}
	}
	return imports
}

// declare reserves a package-level identifier and fails on collisions.
func (g *gen) declare(name, source string) error {
	if prev, ok := g.idents[name]; ok {
		return fmt.Errorf("identifier %s from %s collides with %s", name, source, prev)
	}
	g.idents[name] = source
	return nil
}

func (g *gen) types() error {
	names := sortedKeys(g.spec.Components.Schemas)
	for _, name := range names {
		if err := g.declare(name, "components.schemas."+name); err != nil {
			return err
		}
	}
	for _, name := range names {
		if err := g.declareSchema(g.spec.Components.Schemas[name], name, "components.schemas."+name); err != nil {
			return err
		}
	}
	// Every single-valued "type" property is a JSON:API resource type; they
	// share one enum instead of one type per resource.
	if err := g.declare("ResourceType", "type properties"); err != nil {
		return err
	}
	d := &decl{Name: "ResourceType", Source: "the single-valued \"type\" properties", Kind: "enum"}
	if err := g.enumValues(d, sortedKeys(g.resourceTypes)); err != nil {
		return err
	}
	if err := g.declare("ResourceIdentifier", "inline {type, id} objects"); err != nil {
		return err
	}
	g.decls = append(g.decls, d, &decl{
		Name:   "ResourceIdentifier",
		Source: "the inline objects with exactly the required properties type and id",
		Kind:   "struct",
		Fields: []field{{Name: "Type", Type: "ResourceType", JSONName: "type"}, {Name: "ID", Type: "string", JSONName: "id"}},
	})
	return nil
}

// declareSchema emits the declaration for a schema whose name is already
// reserved: a struct, an enum, or a defined type over another type.
func (g *gen) declareSchema(s *schema, name, source string) error {
	switch {
	case len(s.Enum) > 0:
		d := &decl{Name: name, Source: source, Kind: "enum"}
		g.decls = append(g.decls, d)
		return g.enumValues(d, s.Enum)
	case s.isStruct():
		d := &decl{Name: name, Source: source, Kind: "struct"}
		g.decls = append(g.decls, d)
		return g.structFields(d, s, name)
	}
	t, err := g.goType(s, name, source)
	if err != nil {
		return err
	}
	g.decls = append(g.decls, &decl{Name: name, Source: source, Kind: "def", Type: t})
	return nil
}

func (g *gen) enumValues(d *decl, values []string) error {
	for _, v := range values {
		c := d.Name + pascal(v)
		if err := g.declare(c, d.Source+" value "+v); err != nil {
			return err
		}
		d.Values = append(d.Values, enumValue{c, v})
	}
	return nil
}

// goType returns the Go type for s. Inline enums and objects are declared as
// named types called name; source is their spec location.
func (g *gen) goType(s *schema, name, source string) (string, error) {
	if s.Ref != "" {
		ref, ok := strings.CutPrefix(s.Ref, schemaRefPrefix)
		if !ok || g.spec.Components.Schemas[ref] == nil {
			return "", fmt.Errorf("%s: unresolved $ref %s", source, s.Ref)
		}
		return ref, nil
	}
	if t, ok := g.memo[s]; ok {
		return t, nil
	}
	if typ, ok := identifierType(s); ok {
		g.resourceTypes[typ] = true
		return "ResourceIdentifier", nil
	}
	switch {
	case s.Discriminator != nil && s.Discriminator.PropertyName == "type":
		return "IncludedResource", nil
	case len(s.OneOf) > 0 && !g.allObjects(s.OneOf):
		return "json.RawMessage", nil
	case len(s.Enum) > 0 || s.isStruct():
		if err := g.declare(name, source); err != nil {
			return "", err
		}
		g.memo[s] = name
		return name, g.declareSchema(s, name, source)
	}
	switch s.Type {
	case "array":
		if s.Items == nil {
			return "", fmt.Errorf("%s: array without items", source)
		}
		t, err := g.goType(s.Items, name+"Item", source+".items")
		return "[]" + t, err
	case "object":
		if s.AdditionalProperties == nil {
			return "map[string]any", nil
		}
		t, err := g.goType(s.AdditionalProperties, name+"Value", source+".additionalProperties")
		return "map[string]" + t, err
	case "string":
		if s.Format == "date-time" {
			return "time.Time", nil
		}
		return "string", nil
	case "integer":
		if s.Format == "int64" {
			return "int64", nil
		}
		return "int", nil
	case "number":
		return "float64", nil
	case "boolean":
		return "bool", nil
	case "":
		return "any", nil
	}
	return "", fmt.Errorf("%s: unsupported schema type %q", source, s.Type)
}

// identifierType reports whether s is an inline JSON:API resource
// identifier, exactly the required properties type (one enum value) and id,
// and returns its resource type.
func identifierType(s *schema) (string, bool) {
	if len(s.Properties) != 2 || !slices.Contains(s.Required, "type") || !slices.Contains(s.Required, "id") {
		return "", false
	}
	var typ string
	for _, p := range s.Properties {
		switch {
		case p.Name == "type" && len(p.Schema.Enum) == 1:
			typ = p.Schema.Enum[0]
		case p.Name == "id" && p.Schema.Type == "string":
		default:
			return "", false
		}
	}
	return typ, typ != ""
}

func (s *schema) isStruct() bool {
	return len(s.Properties) > 0 || len(s.AllOf) > 0 || len(s.OneOf) > 0
}

func (g *gen) resolve(s *schema) *schema {
	if ref, ok := strings.CutPrefix(s.Ref, schemaRefPrefix); ok {
		return g.spec.Components.Schemas[ref]
	}
	return s
}

func (g *gen) allObjects(ss []*schema) bool {
	for _, s := range ss {
		if r := g.resolve(s); !r.isStruct() && r.Type != "object" {
			return false
		}
	}
	return true
}

// member is a property collected for a struct, with the name of the schema
// that defines it (used to name its inline types).
type member struct {
	owner    string
	prop     property
	required bool
}

// collect gathers the properties of s, merging allOf parts and oneOf
// variants. oneOf variants contribute optional fields only.
func (g *gen) collect(s *schema, owner string, required bool, out *[]member) {
	if ref, ok := strings.CutPrefix(s.Ref, schemaRefPrefix); ok {
		g.collect(g.spec.Components.Schemas[ref], ref, required, out)
		return
	}
	for _, p := range s.AllOf {
		g.collect(p, owner, required, out)
	}
	for _, p := range s.OneOf {
		g.collect(p, owner, false, out)
	}
	for _, p := range s.Properties {
		*out = append(*out, member{owner, p, required && slices.Contains(s.Required, p.Name)})
	}
}

func (g *gen) structFields(d *decl, s *schema, name string) error {
	var members []member
	g.collect(s, name, true, &members)
	seen := map[string]bool{}
	for _, m := range members {
		if seen[m.prop.Name] {
			continue // first definition wins, as in merged oneOf variants
		}
		seen[m.prop.Name] = true
		fname := pascal(m.prop.Name)
		source := m.owner + "." + m.prop.Name
		var t string
		if ps := m.prop.Schema; m.prop.Name == "type" && ps.Ref == "" && len(ps.Enum) == 1 {
			g.resourceTypes[ps.Enum[0]] = true
			t = "ResourceType"
		} else {
			var err error
			if t, err = g.goType(ps, m.owner+fname, source); err != nil {
				return err
			}
		}
		// Nullable fields need three states (unset, null, value): null is how
		// the API clears a field.
		var omit string
		switch {
		case m.prop.Schema.Nullable || g.resolve(m.prop.Schema).Nullable:
			t = "Nullable[" + t + "]"
			if !m.required {
				omit = "omitzero"
			}
		case !m.required:
			omit = "omitempty"
			if pointable(t) {
				t = "*" + t
			}
		}
		for _, f := range d.Fields {
			if f.Name == fname {
				return fmt.Errorf("%s: field %s collides with %s", source, fname, f.JSONName)
			}
		}
		d.Fields = append(d.Fields, field{Name: fname, Type: t, JSONName: m.prop.Name, Omit: omit})
	}
	return nil
}

// pointable reports whether an optional field of type t needs a pointer to
// tell "unset" apart from the zero value.
func pointable(t string) bool {
	return !strings.HasPrefix(t, "[]") && !strings.HasPrefix(t, "map[") && t != "any" && t != "json.RawMessage"
}

type operation struct {
	ID, Name, Method, Path, PathExpr string
	Params                           []param
	Query                            []param
	Body, Result                     string
}

type param struct {
	Name, GoName, In, Type, Setter string
	Required                       bool
}

type rawParam struct {
	Ref      string  `json:"$ref"`
	Name     string  `json:"name"`
	In       string  `json:"in"`
	Required bool    `json:"required"`
	Style    string  `json:"style"`
	Explode  *bool   `json:"explode"`
	Schema   *schema `json:"schema"`
}

type rawOperation struct {
	OperationID string     `json:"operationId"`
	Parameters  []rawParam `json:"parameters"`
	RequestBody *struct {
		Content content `json:"content"`
	} `json:"requestBody"`
	Responses map[string]struct {
		Ref     string  `json:"$ref"`
		Content content `json:"content"`
	} `json:"responses"`
}

// content maps media types to schemas.
type content map[string]struct {
	Schema *schema `json:"schema"`
}

func (g *gen) operations() ([]operation, error) {
	var ops []operation
	for _, path := range sortedKeys(g.spec.Paths) {
		item := g.spec.Paths[path]
		var shared []rawParam
		if raw, ok := item["parameters"]; ok {
			if err := json.Unmarshal(raw, &shared); err != nil {
				return nil, fmt.Errorf("%s parameters: %w", path, err)
			}
		}
		for _, method := range sortedKeys(item) {
			if method == "parameters" {
				continue
			}
			var raw rawOperation
			if err := json.Unmarshal(item[method], &raw); err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			op, err := g.operation(path, strings.ToUpper(method), raw, append(slices.Clone(shared), raw.Parameters...))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", raw.OperationID, err)
			}
			ops = append(ops, op)
		}
	}
	slices.SortFunc(ops, func(a, b operation) int { return strings.Compare(a.Name, b.Name) })
	return ops, nil
}

func (g *gen) operation(path, method string, raw rawOperation, params []rawParam) (operation, error) {
	op := operation{ID: raw.OperationID, Name: pascal(raw.OperationID), Method: method, Path: path}
	pathArgs := map[string]string{}
	for _, rp := range params {
		if rp.Ref != "" {
			return op, fmt.Errorf("parameter $ref %s is not supported", rp.Ref)
		}
		if rp.Schema == nil {
			return op, fmt.Errorf("parameter %s has no inline schema", rp.Name)
		}
		p := param{Name: rp.Name, GoName: pascal(rp.Name), In: rp.In, Required: rp.Required}
		for _, prev := range op.Params {
			if prev.GoName == p.GoName {
				return op, fmt.Errorf("parameters %s and %s collide", prev.Name, p.Name)
			}
		}
		switch {
		case rp.In == "path":
			p.Type = "string"
			pathArgs[rp.Name] = "url.PathEscape(params." + p.GoName + ")"
		case rp.In == "query" && rp.Schema.Type == "array":
			// setList joins the values with commas
			if rp.Style != "" && rp.Style != "form" || rp.Explode == nil || *rp.Explode {
				return op, fmt.Errorf("query parameter %s: only style form with explode false (comma separated) is supported", rp.Name)
			}
			p.Type, p.Setter = "[]string", "List"
		case rp.In == "query" && rp.Schema.Type == "integer":
			p.Type, p.Setter = "int", "Int"
		case rp.In == "query" && rp.Schema.Type == "boolean":
			p.Type, p.Setter = "*bool", "Bool"
		default:
			return op, fmt.Errorf("unsupported %s parameter %s of type %s", rp.In, rp.Name, rp.Schema.Type)
		}
		op.Params = append(op.Params, p)
		if p.Setter != "" {
			op.Query = append(op.Query, p)
		}
	}
	if len(op.Params) > 0 {
		if err := g.declare(op.Name+"Params", raw.OperationID+" parameters"); err != nil {
			return op, err
		}
	}

	var expr []string
	for rest := path; rest != ""; {
		before, after, ok := strings.Cut(rest, "{")
		if before != "" {
			expr = append(expr, fmt.Sprintf("%q", before))
		}
		if !ok {
			break
		}
		name, tail, _ := strings.Cut(after, "}")
		arg, ok := pathArgs[name]
		if !ok {
			return op, fmt.Errorf("path parameter %s is not declared", name)
		}
		expr = append(expr, arg)
		rest = tail
	}
	op.PathExpr = strings.Join(expr, " + ")

	if raw.RequestBody != nil {
		t, err := g.jsonRef(raw.RequestBody.Content, "request body")
		if err != nil {
			return op, err
		}
		op.Body = t
	}
	for _, code := range sortedKeys(raw.Responses) {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		if ref := raw.Responses[code].Ref; ref != "" {
			return op, fmt.Errorf("response %s: $ref %s is not supported", code, ref)
		}
		if len(raw.Responses[code].Content) > 0 {
			t, err := g.jsonRef(raw.Responses[code].Content, "response "+code)
			if err != nil {
				return op, err
			}
			op.Result = t
		}
		break
	}
	return op, nil
}

// jsonRef returns the type of an application/json body, which must be a $ref.
func (g *gen) jsonRef(c content, what string) (string, error) {
	media, ok := c["application/json"]
	if !ok || len(c) != 1 || media.Schema == nil || media.Schema.Ref == "" {
		return "", fmt.Errorf("%s: only a single application/json $ref is supported", what)
	}
	return g.goType(media.Schema, "", what)
}

// pascal turns a spec name into an exported Go identifier: "whatsNew" ->
// "WhatsNew", "filter[app]" -> "FilterApp", "APP_IPHONE_67" -> "AppIphone67",
// "builds_getCollection" -> "BuildsGetCollection". The words Id and Url
// become ID and URL.
func pascal(s string) string {
	var b strings.Builder
	segs := strings.FieldsFunc(s, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9')
	})
	for _, seg := range segs {
		if strings.ToUpper(seg) == seg { // UPPER_SNAKE value
			seg = strings.ToLower(seg)
		}
		for _, w := range camelWords(seg) {
			w = strings.ToUpper(w[:1]) + w[1:]
			switch w {
			case "Id":
				w = "ID"
			case "Url":
				w = "URL"
			}
			b.WriteString(w)
		}
	}
	return b.String()
}

// camelWords splits "preReleaseVersion" into "pre", "Release", "Version".
func camelWords(s string) []string {
	var words []string
	start := 0
	for i := 1; i < len(s); i++ {
		if isUpper(s[i]) && !isUpper(s[i-1]) {
			words = append(words, s[start:i])
			start = i
		}
	}
	return append(words, s[start:])
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
