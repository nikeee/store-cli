package media

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
)

func isImageFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg":
		return true
	}
	return false
}

// imageFiles returns the image files directly in dir, sorted by name.
// A missing dir yields no files.
func imageFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, clierr.New("%v", err)
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() && isImageFile(e.Name()) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// ScanAppleScreenshots returns locale -> device alias -> sorted file paths.
// Files directly in <dir>/<locale>/ get their device from the pixel size
// (for platform); files in <dir>/<locale>/<device-alias>/ take that alias.
// A file that is no image or fits no device and an unknown device folder go
// to c and are left out; the error is only for unreadable directories.
func ScanAppleScreenshots(dir, platform string, c *clierr.Collector) (map[string]map[string][]string, error) {
	out := map[string]map[string][]string{}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, clierr.New("%v", err)
	}
	for _, e := range entries {
		if !e.IsDir() || !metadata.IsLocale(e.Name()) {
			continue
		}
		locale := e.Name()
		locDir := filepath.Join(dir, locale)
		sets := map[string][]string{}
		files, err := imageFiles(locDir)
		if err != nil {
			return nil, err
		}
		for _, p := range files {
			info, err := Inspect(p)
			if err != nil {
				c.Add(err)
				continue
			}
			d, err := DetectDevice(platform, info.Width, info.Height)
			if err != nil {
				c.Add(clierr.Filef(p, "%v", err))
				continue
			}
			sets[d.Alias] = append(sets[d.Alias], p)
		}
		subs, err := os.ReadDir(locDir)
		if err != nil {
			return nil, clierr.New("%v", err)
		}
		for _, s := range subs {
			if !s.IsDir() || strings.HasPrefix(s.Name(), ".") {
				continue
			}
			if _, ok := DeviceByAlias(s.Name()); !ok {
				c.Add(clierr.New("%s: unknown device folder %q", filepath.Join(locDir, s.Name()), s.Name()))
				continue
			}
			files, err := imageFiles(filepath.Join(locDir, s.Name()))
			if err != nil {
				return nil, err
			}
			sets[s.Name()] = append(sets[s.Name()], files...)
		}
		for alias := range sets {
			sort.Strings(sets[alias])
		}
		if len(sets) > 0 {
			out[locale] = sets
		}
	}
	return out, nil
}

// ScanGoogleImages returns Play locale -> image type alias -> sorted file
// paths from <dir>/android/<locale>/images/. An image file or folder there
// that is no image type would be left out, so it goes to c; other files are
// ignored. The error is only for unreadable directories.
func ScanGoogleImages(dir string, c *clierr.Collector) (map[string]map[string][]string, error) {
	out := map[string]map[string][]string{}
	root := filepath.Join(dir, metadata.AndroidDir)
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, clierr.New("%v", err)
	}
	for _, e := range entries {
		if !e.IsDir() || !metadata.IsLocale(e.Name()) {
			continue
		}
		sets := map[string][]string{}
		for _, t := range ImageTypes {
			p := filepath.Join(root, e.Name(), filepath.FromSlash(t.Path))
			if t.Multiple {
				files, err := imageFiles(p)
				if err != nil {
					return nil, err
				}
				if len(files) > 0 {
					sets[t.Alias] = files
				}
			} else if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
				sets[t.Alias] = []string{p}
			}
		}
		if err := unknownImages(filepath.Join(root, e.Name(), "images"), c); err != nil {
			return nil, err
		}
		if len(sets) > 0 {
			out[e.Name()] = sets
		}
	}
	return out, nil
}

// unknownImages adds to c every folder and image file in dir that is no
// image type.
func unknownImages(dir string, c *clierr.Collector) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return clierr.New("%v", err)
	}
	var names []string
	for _, t := range ImageTypes {
		names = append(names, path.Base(t.Path))
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || slices.Contains(names, name) {
			continue
		}
		p := filepath.Join(dir, name)
		switch {
		case e.IsDir():
			c.Add(clierr.New("%s: unknown image folder %q (use %s)", p, name, strings.Join(names, ", ")))
		case isImageFile(name):
			c.Add(clierr.Filef(p, "unknown image file (use %s)", strings.Join(names, ", ")))
		}
	}
	return nil
}
