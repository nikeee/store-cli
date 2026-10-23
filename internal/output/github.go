package output

import (
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nikeee/store-cli/internal/clierr"
)

// AnnotateGitHub makes the printer also write errors and warnings as GitHub
// Actions workflow commands, which show as annotations. Files are named
// relative to workspace, where GitHub looks for them.
func (p *Printer) AnnotateGitHub(workspace string) {
	p.github, p.workspace = true, workspace
}

// Escaping of workflow commands, see
// https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands
var (
	escapeData     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	escapeProperty = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

// annotateError writes one error annotation per issue of err, else one for
// the whole error.
func (p *Printer) annotateError(err error, body errorBody) {
	var ce *clierr.Error
	if errors.As(err, &ce) && len(ce.Issues) > 0 {
		for _, is := range ce.Issues {
			p.annotate("error", is)
		}
		return
	}
	msg := body.Error
	if body.Detail != "" {
		msg += "\n" + body.Detail
	}
	if body.Hint != "" {
		msg += "\nhint: " + body.Hint
	}
	p.annotate("error", clierr.Issue{Message: msg})
}

// annotate writes the workflow command "::<level> file=..,line=..,col=..::<message>".
func (p *Printer) annotate(level string, is clierr.Issue) {
	var props []string
	if is.File != "" {
		props = append(props, "file="+escapeProperty.Replace(p.workspacePath(is.File)))
		if is.Line > 0 {
			props = append(props, "line="+strconv.Itoa(is.Line))
			if is.Col > 0 {
				props = append(props, "col="+strconv.Itoa(is.Col))
			}
		}
	}
	cmd := "::" + level
	if len(props) > 0 {
		cmd += " " + strings.Join(props, ",")
	}
	_, _ = io.WriteString(p.stderr, cmd+"::"+escapeData.Replace(is.Message)+"\n")
}

// workspacePath returns path relative to the workspace. A path outside of it
// stays as given.
func (p *Printer) workspacePath(path string) string {
	if p.workspace == "" {
		return filepath.ToSlash(path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(p.workspace, abs)
	if err != nil || !filepath.IsLocal(rel) {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// AnnotateWarning writes is as a warning annotation with AnnotateGitHub;
// otherwise nothing.
func (p *Printer) AnnotateWarning(is clierr.Issue) {
	if p.github {
		p.annotate("warning", is)
	}
}
