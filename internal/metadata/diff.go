package metadata

// Action is what a sync step does to the store.
type Action string

const (
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionReplace Action = "replace"
	ActionSkip    Action = "skip"
	ActionDelete  Action = "delete"
)

// Change is one planned or applied modification.
type Change struct {
	Action Action `json:"action"`
	Scope  string `json:"scope"` // "app-info" | "version" | "review" | "screenshots" | "listing" | "images"
	Locale string `json:"locale,omitempty"`
	Field  string `json:"field,omitempty"`
	Old    string `json:"old,omitempty"`
	New    string `json:"new,omitempty"`
}

// Plan is the list of changes a command would make.
type Plan struct {
	Changes []Change `json:"changes"`
}

// DiffTexts compares the fields of the given level present in local with
// remote. remote == nil means the locale does not exist remotely, so every
// local field is a create. Changes are in field table order.
func DiffTexts(scope Level, locale string, fields []Field, local, remote Texts) []Change {
	var out []Change
	for _, f := range fields {
		if f.Level != scope {
			continue
		}
		nv, ok := local[f.API]
		if !ok {
			continue
		}
		c := Change{Scope: string(scope), Locale: locale, Field: f.API, New: nv}
		if remote == nil {
			c.Action = ActionCreate
			out = append(out, c)
			continue
		}
		if ov := remote[f.API]; ov != nv {
			c.Action = ActionUpdate
			c.Old = ov
			out = append(out, c)
		}
	}
	return out
}
