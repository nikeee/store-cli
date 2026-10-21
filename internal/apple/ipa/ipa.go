// Package ipa reads the app metadata App Store Connect needs from an IPA
// without macOS tools.
package ipa

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"path"

	"howett.net/plist"
)

// Info is the metadata of the app inside an IPA.
type Info struct {
	BundleID string
	Version  string
	Build    string
	MinOS    string
	// Platform is in App Store Connect API form: IOS, TV_OS or VISION_OS.
	Platform string
}

type infoPlist struct {
	BundleID       string `plist:"CFBundleIdentifier"`
	Version        string `plist:"CFBundleShortVersionString"`
	Build          string `plist:"CFBundleVersion"`
	MinOS          string `plist:"MinimumOSVersion"`
	DTPlatformName string `plist:"DTPlatformName"`
}

// Read returns the metadata from Payload/<name>.app/Info.plist of the IPA at path.
func Read(ipaPath string) (Info, error) {
	zr, err := zip.OpenReader(ipaPath)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) {
			return Info{}, fmt.Errorf("%s is not an IPA (not a zip archive)", ipaPath)
		}
		return Info{}, fmt.Errorf("open %s: %w", ipaPath, err)
	}
	defer zr.Close()

	var found []*zip.File
	for _, f := range zr.File {
		if isAppPlist(f.Name) {
			found = append(found, f)
		}
	}
	switch len(found) {
	case 0:
		return Info{}, fmt.Errorf("%s: no Payload/<name>.app/Info.plist found", ipaPath)
	case 1:
	default:
		return Info{}, fmt.Errorf("%s: several app Info.plist files found (%d)", ipaPath, len(found))
	}

	data, err := readFile(found[0])
	if err != nil {
		return Info{}, fmt.Errorf("%s: read %s: %w", ipaPath, found[0].Name, err)
	}
	var p infoPlist
	if _, err := plist.Unmarshal(data, &p); err != nil {
		return Info{}, fmt.Errorf("%s: decode %s: %w", ipaPath, found[0].Name, err)
	}
	for _, k := range []struct{ key, val string }{
		{"CFBundleIdentifier", p.BundleID},
		{"CFBundleShortVersionString", p.Version},
		{"CFBundleVersion", p.Build},
	} {
		if k.val == "" {
			return Info{}, fmt.Errorf("%s: %s is missing in Info.plist", ipaPath, k.key)
		}
	}
	return Info{
		BundleID: p.BundleID,
		Version:  p.Version,
		Build:    p.Build,
		MinOS:    p.MinOS,
		Platform: platform(p.DTPlatformName),
	}, nil
}

// isAppPlist reports whether name is exactly Payload/<name>.app/Info.plist.
func isAppPlist(name string) bool {
	dir, file := path.Split(name)
	if file != "Info.plist" {
		return false
	}
	appDir, app := path.Split(path.Clean(dir))
	return appDir == "Payload/" && path.Ext(app) == ".app" && app != ".app"
}

func readFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func platform(dtPlatformName string) string {
	switch dtPlatformName {
	case "appletvos":
		return "TV_OS"
	case "xros":
		return "VISION_OS"
	default:
		return "IOS"
	}
}
