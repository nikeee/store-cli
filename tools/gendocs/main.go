// Command gendocs writes the command reference: one page per command from
// the cobra tree, an index, and tables.md from the code tables, plus the
// command list of the agent skill. Run it with go generate (see
// internal/cli/doc.go). The output is deterministic.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/nikeee/store-cli/internal/cli"
	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/locale"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
)

func main() {
	out := flag.String("out", "docs/reference", "output directory")
	skill := flag.String("skill", "skills/store-cli/references/commands.md", "command list of the agent skill")
	flag.Parse()
	if err := run(*out); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*skill, []byte(skillCommands(cli.NewRootCmd(cli.DefaultDeps()))), 0o644); err != nil {
		log.Fatal(err)
	}
}

func run(out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	// Remove old pages so deleted commands do not linger.
	old, err := filepath.Glob(filepath.Join(out, "*.md"))
	if err != nil {
		return err
	}
	for _, f := range old {
		if err := os.Remove(f); err != nil {
			return err
		}
	}

	root := cli.NewRootCmd(cli.DefaultDeps())
	root.DisableAutoGenTag = true
	resultParagraphs(root)
	if err := doc.GenMarkdownTree(root, out); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "README.md"), []byte(index(root)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "results.md"), []byte(resultsPage()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "tables.md"), []byte(tables()), 0o644)
}

// resultParagraphs puts the final Result line of every Long text in its own
// paragraph, so Markdown does not join it to the text above.
func resultParagraphs(c *cobra.Command) {
	if i := strings.LastIndex(c.Long, "\nResult: "); i >= 0 && !strings.Contains(c.Long[i+1:], "\n") {
		c.Long = c.Long[:i] + "\n" + c.Long[i:]
	}
	for _, sub := range c.Commands() {
		resultParagraphs(sub)
	}
}

func pageName(c *cobra.Command) string {
	return strings.ReplaceAll(c.CommandPath(), " ", "_") + ".md"
}

// index lists every command with its short text, nested like the tree.
func index(root *cobra.Command) string {
	var b bytes.Buffer
	b.WriteString("# store - command reference\n\n")
	b.WriteString("Generated from the command help by `go generate ./...`; do not edit.\n")
	b.WriteString("Lookup tables (fields, devices, image types, locales, environment variables) are in [tables.md](tables.md).\n")
	b.WriteString("The JSON result objects named in the `Result:` line of each command are in [results.md](results.md).\n")
	b.WriteString("Conventions, output formats and exit codes are in [../commands.md](../commands.md).\n\n")
	var walk func(c *cobra.Command, depth int)
	walk = func(c *cobra.Command, depth int) {
		fmt.Fprintf(&b, "%s- [%s](%s) - %s\n", strings.Repeat("  ", depth), c.CommandPath(), pageName(c), c.Short)
		for _, s := range c.Commands() {
			if s.IsAvailableCommand() && s.Name() != "help" {
				walk(s, depth+1)
			}
		}
	}
	walk(root, 0)
	return b.String()
}

// skillCommands lists every command with its arguments and summary, grouped
// by top-level command, for the agent skill, which ships without the
// reference pages.
func skillCommands(root *cobra.Command) string {
	var b bytes.Buffer
	b.WriteString("# store commands\n\n")
	b.WriteString("Generated from the command help by `go generate ./...`; do not edit.\n")
	b.WriteString("`store <command> --help` shows the flags, an example and the result of each command.\n")
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Parent() == root {
			fmt.Fprintf(&b, "\n## %s\n\n", c.CommandPath())
		}
		if !c.HasAvailableSubCommands() {
			fmt.Fprintf(&b, "- `%s` - %s\n", strings.TrimSuffix(c.UseLine(), " [flags]"), c.Short)
			return
		}
		for _, s := range c.Commands() {
			if s.IsAvailableCommand() && s.Name() != "help" && s.Name() != "completion" {
				walk(s)
			}
		}
	}
	walk(root)
	return b.String()
}

type table struct {
	title string
	intro string
	head  []string
	rows  [][]string
}

func (t table) render(b *bytes.Buffer) {
	fmt.Fprintf(b, "## %s\n\n", t.title)
	if t.intro != "" {
		b.WriteString(t.intro + "\n\n")
	}
	b.WriteString("| " + strings.Join(t.head, " | ") + " |\n")
	b.WriteString("|" + strings.Repeat("---|", len(t.head)) + "\n")
	for _, r := range t.rows {
		b.WriteString("| " + strings.Join(r, " | ") + " |\n")
	}
	b.WriteString("\n")
}

func code(s string) string {
	if s == "" {
		return "-"
	}
	return "`" + s + "`"
}

func tables() string {
	var b bytes.Buffer
	b.WriteString("# store - tables\n\n")
	b.WriteString("Generated from the code tables by `go generate ./...`; do not edit.\n\n")
	for _, t := range []table{
		envTable(),
		agentTable(),
		fieldTable("App Store listing fields", "Per locale. Files live in `<dir>/<locale>/`; flags belong to `store apple listing set`. `app-info` fields need an editable app info.", metadata.AppleFields),
		fieldTable("App Review information fields", "Files live in `<dir>/review_information/`; flags belong to `store apple review-info set`. The demo password is masked in output.", metadata.AppleReviewFields),
		fieldTable("Google Play listing fields", "Per locale. Files live in `<dir>/android/<locale>/`; flags belong to `store google listing set`.", metadata.GoogleFields),
		deviceTable(),
		imageTypeTable(),
		localeTable(),
	} {
		t.render(&b)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func envTable() table {
	return table{
		title: "Environment variables",
		head:  []string{"Variable", "Used by", "Content"},
		rows: [][]string{
			{code(config.EnvAppleIssuerID), "`store apple`", "Issuer ID from App Store Connect, Users and Access, Integrations"},
			{code(config.EnvAppleKeyID), "`store apple`", "Key ID of the team API key"},
			{code(config.EnvApplePrivateKey), "`store apple`", "The `.p8` key: PEM content, or a path to the file"},
			{code(config.EnvAppleBundleID), "`store apple`", "Default for `--app`"},
			{code(config.EnvGoogleServiceAccount), "`store google`", "Service account key: JSON content, or a path to the file"},
			{code(config.EnvGooglePackageName), "`store google`", "Default for `--app`"},
			{code(config.EnvOutput), "all", "Default for `--output`"},
			{code(config.EnvNoColor), "all", "Disables colors in text output"},
			{code(config.EnvGitHubActions), "all", "`true` (set by GitHub Actions): errors and warnings also become annotations"},
			{code(config.EnvGitHubWorkspace), "all", "Set by GitHub Actions: annotations name files relative to it"},
		},
	}
}

func agentTable() table {
	t := table{
		title: "Agent markers",
		intro: "If any of these environment variables is set, the default output format is `toon` instead of `text`.",
		head:  []string{"Variable"},
	}
	for _, m := range config.AgentMarkers {
		t.rows = append(t.rows, []string{code(m)})
	}
	return t
}

func fieldTable(title, intro string, fields []metadata.Field) table {
	t := table{title: title, intro: intro, head: []string{"File", "Flag", "API field", "Level", "Max chars"}}
	for _, f := range fields {
		flagCell := "-"
		if f.Flag != "" {
			flagCell = code("--" + f.Flag)
		}
		maxChars := "-"
		if f.MaxChars > 0 {
			maxChars = fmt.Sprint(f.MaxChars)
		}
		t.rows = append(t.rows, []string{code(f.File), flagCell, code(f.API), string(f.Level), maxChars})
	}
	return t
}

func deviceTable() table {
	t := table{
		title: "App Store screenshot and preview devices",
		intro: "`--device` aliases. Sizes are portrait width x height pixels; landscape is accepted too. " +
			"Where a size fits several devices of a platform, the first row wins. Watch devices have no previews.",
		head: []string{"Alias", "Display type", "Preview type", "Platform", "Sizes"},
	}
	for _, d := range media.Devices {
		sizes := make([]string, len(d.Sizes))
		for i, s := range d.Sizes {
			sizes[i] = fmt.Sprintf("%dx%d", s[0], s[1])
		}
		t.rows = append(t.rows, []string{code(d.Alias), code(d.DisplayType), code(d.PreviewType), d.Platform, strings.Join(sizes, ", ")})
	}
	return t
}

func imageTypeTable() table {
	t := table{
		title: "Google Play image types",
		intro: "`--type` aliases. Paths are relative to `<dir>/android/<locale>/`; a folder holds several images in file name order.",
		head:  []string{"`--type`", "Folder or file", "API image type", "Count", "Size", "Alpha"},
	}
	for _, it := range media.ImageTypes {
		count := "1"
		switch {
		case it.Multiple && it.Min > 0:
			count = fmt.Sprintf("%d to %d", it.Min, it.Max)
		case it.Multiple:
			count = fmt.Sprintf("up to %d", it.Max)
		}
		var size string
		switch {
		case it.Width > 0:
			size = fmt.Sprintf("%dx%d", it.Width, it.Height)
		case it.MaxAspect == 1:
			size = fmt.Sprintf("square, %d-%d px per side", it.MinSide, it.MaxSide)
		default:
			size = fmt.Sprintf("%d-%d px per side, long side at most %gx the short side", it.MinSide, it.MaxSide, it.MaxAspect)
		}
		alpha := "no"
		if it.AllowAlpha {
			alpha = "yes"
		}
		t.rows = append(t.rows, []string{code(it.Alias), code(it.Path), code(it.APIType), count, size, alpha})
	}
	return t
}

func localeTable() table {
	t := table{
		title: "Locale mapping",
		intro: "App Store locales whose Google Play language code differs. Every other locale is the same in both stores. " +
			"The Google commands take either form and write Play codes.",
		head: []string{"App Store", "Google Play"},
	}
	for _, m := range locale.Mappings() {
		t.rows = append(t.rows, []string{code(m.AppStore), code(m.Play)})
	}
	return t
}
