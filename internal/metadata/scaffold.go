package metadata

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nikeee/store-cli/internal/clierr"
)

// InitResult is the result of store apple init and store google init.
type InitResult struct {
	Created  []string `json:"created"`
	Existing []string `json:"existing"`
}

// Merge appends the paths of other.
func (r *InitResult) Merge(other InitResult) {
	r.Created = append(r.Created, other.Created...)
	r.Existing = append(r.Existing, other.Existing...)
}

// ScaffoldDir creates one empty file per field directly in dir. Existing
// files are left alone and reported in Existing.
func ScaffoldDir(dir string, fields []Field) (InitResult, error) {
	res := InitResult{Created: []string{}, Existing: []string{}}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, clierr.New("%v", err)
	}
	for _, f := range fields {
		p := filepath.Join(dir, f.File)
		file, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		switch {
		case errors.Is(err, fs.ErrExist):
			res.Existing = append(res.Existing, p)
		case err != nil:
			return res, clierr.New("%v", err)
		default:
			if err := file.Close(); err != nil {
				return res, clierr.New("%v", err)
			}
			res.Created = append(res.Created, p)
		}
	}
	return res, nil
}

// Scaffold creates dir/<locale>/<file> for every locale and field.
func Scaffold(dir string, fields []Field, locales []string) (InitResult, error) {
	res := InitResult{Created: []string{}, Existing: []string{}}
	for _, l := range locales {
		r, err := ScaffoldDir(filepath.Join(dir, l), fields)
		res.Merge(r)
		if err != nil {
			return res, err
		}
	}
	return res, nil
}
