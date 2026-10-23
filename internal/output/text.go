package output

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var timeType = reflect.TypeOf(time.Time{})

// renderGeneric renders any value without a TextRenderer: structs and maps as
// key: value lines, slices of structs as tables.
func renderGeneric(w io.Writer, s Styles, v any) error {
	var sb strings.Builder
	writeValue(&sb, s, reflect.ValueOf(v), "")
	_, err := io.WriteString(w, sb.String())
	return err
}

func deref(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func isScalar(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		return t == timeType
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Interface:
		return false
	}
	return true
}

func isEmpty(v reflect.Value) bool {
	v = deref(v)
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	}
	if v.Type() == timeType {
		return v.Interface().(time.Time).IsZero()
	}
	return false
}

func scalarString(v reflect.Value) string {
	v = deref(v)
	if v.Type() == timeType {
		return v.Interface().(time.Time).Format(time.RFC3339)
	}
	return fmt.Sprint(v.Interface())
}

// cell renders a scalar or a slice of scalars on one line.
func cell(v reflect.Value) string {
	v = deref(v)
	if !v.IsValid() {
		return ""
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		parts := make([]string, v.Len())
		for i := range parts {
			parts[i] = scalarString(v.Index(i))
		}
		return strings.Join(parts, ", ")
	}
	return scalarString(v)
}

// isCell reports whether a field fits on one line: a scalar or a slice of scalars.
func isCell(t reflect.Type) bool {
	if isScalar(t) {
		return true
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) && isScalar(t.Elem())
}

type entry struct {
	key string
	v   reflect.Value
}

// jsonField returns the json name of a struct field, or "" if it is skipped.
func jsonField(f reflect.StructField) string {
	if !f.IsExported() {
		return ""
	}
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "-" {
		return ""
	}
	if name == "" {
		return f.Name
	}
	return name
}

// field is a struct field as json writes it: its name and its index path
// (several steps for a field of an embedded struct).
type field struct {
	name  string
	index []int
}

// jsonFields returns the fields of struct type t that json writes, with the
// fields of embedded untagged structs inlined as json does.
func jsonFields(t reflect.Type) []field {
	var out []field
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if f.Anonymous && f.Tag.Get("json") == "" && ft.Kind() == reflect.Struct {
			for _, sub := range jsonFields(ft) {
				out = append(out, field{sub.name, append([]int{i}, sub.index...)})
			}
			continue
		}
		if name := jsonField(f); name != "" {
			out = append(out, field{name, []int{i}})
		}
	}
	return out
}

// fieldOf returns the field at index of v; the zero Value when a nil
// embedded pointer is in the way.
func fieldOf(v reflect.Value, index []int) reflect.Value {
	f, err := v.FieldByIndexErr(index)
	if err != nil {
		return reflect.Value{}
	}
	return f
}

func entries(v reflect.Value) []entry {
	var out []entry
	switch v.Kind() {
	case reflect.Struct:
		for _, f := range jsonFields(v.Type()) {
			out = append(out, entry{f.name, fieldOf(v, f.index)})
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			out = append(out, entry{fmt.Sprint(k.Interface()), v.MapIndex(k)})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	}
	return out
}

func writeValue(sb *strings.Builder, s Styles, v reflect.Value, indent string) {
	v = deref(v)
	if !v.IsValid() {
		sb.WriteString(indent + s.Dim.Render("(none)") + "\n")
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == timeType {
			break
		}
		fallthrough
	case reflect.Map:
		writeEntries(sb, s, v, indent)
		return
	case reflect.Slice, reflect.Array:
		writeList(sb, s, v, indent)
		return
	}
	sb.WriteString(indent + scalarString(v) + "\n")
}

func writeEntries(sb *strings.Builder, s Styles, v reflect.Value, indent string) {
	for _, e := range entries(v) {
		if isEmpty(e.v) {
			continue
		}
		key := s.Key.Render(e.key + ":")
		if isCell(e.v.Type()) || (deref(e.v).Kind() == reflect.Interface) {
			if inner := deref(e.v); inner.IsValid() && !isCell(inner.Type()) {
				sb.WriteString(indent + key + "\n")
				writeValue(sb, s, inner, indent+"  ")
				continue
			}
			sb.WriteString(indent + key + " " + cell(e.v) + "\n")
			continue
		}
		sb.WriteString(indent + key + "\n")
		writeValue(sb, s, e.v, indent+"  ")
	}
}

func writeList(sb *strings.Builder, s Styles, v reflect.Value, indent string) {
	if v.Len() == 0 {
		sb.WriteString(indent + s.Dim.Render("(none)") + "\n")
		return
	}
	first := deref(v.Index(0))
	if !first.IsValid() || first.Kind() != reflect.Struct || first.Type() == timeType {
		for i := 0; i < v.Len(); i++ {
			writeValue(sb, s, v.Index(i), indent)
		}
		return
	}
	writeTable(sb, s, v, indent)
}

func writeTable(sb *strings.Builder, s Styles, v reflect.Value, indent string) {
	elem := deref(v.Index(0)).Type()
	var cols, nested []field
	for _, f := range jsonFields(elem) {
		if isCell(elem.FieldByIndex(f.index).Type) {
			cols = append(cols, f)
		} else {
			nested = append(nested, f)
		}
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	rows := make([][]string, v.Len())
	for r := range rows {
		item := deref(v.Index(r))
		for _, c := range cols {
			rows[r] = append(rows[r], cell(fieldOf(item, c.index)))
		}
	}
	head := make([]string, len(names))
	for i, n := range names {
		head[i] = strings.ToUpper(n)
	}
	t := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderHeader(false).BorderColumn(false).
		Headers(head...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return s.Key.PaddingRight(2)
			}
			return lipgloss.NewStyle().PaddingRight(2)
		})
	for _, line := range strings.Split(t.String(), "\n") {
		sb.WriteString(indent + strings.TrimRight(line, " ") + "\n")
	}
	for r := 0; r < v.Len(); r++ {
		item := deref(v.Index(r))
		for _, f := range nested {
			fv := fieldOf(item, f.index)
			if !fv.IsValid() || isEmpty(fv) {
				continue
			}
			label := f.name
			if len(rows[r]) > 0 && rows[r][0] != "" {
				label = rows[r][0] + " " + label
			}
			sb.WriteString("\n" + indent + s.Key.Render(label+":") + "\n")
			writeValue(sb, s, fv, indent+"  ")
		}
	}
}
