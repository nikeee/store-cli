package output

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Styles are the text styles used by renderers. Without color every style
// renders its input unchanged.
type Styles struct {
	Key   lipgloss.Style // field names and table headers
	Dim   lipgloss.Style // placeholders and progress
	Error lipgloss.Style
	Warn  lipgloss.Style
	Color bool
}

// NewStyles returns the styles for color or plain output.
func NewStyles(color bool) Styles {
	r := lipgloss.NewRenderer(io.Discard)
	if color {
		r.SetColorProfile(termenv.ANSI256)
	} else {
		r.SetColorProfile(termenv.Ascii)
	}
	return Styles{
		Key:   r.NewStyle().Bold(true).Foreground(lipgloss.Color("6")),
		Dim:   r.NewStyle().Faint(true),
		Error: r.NewStyle().Foreground(lipgloss.Color("1")),
		Warn:  r.NewStyle().Foreground(lipgloss.Color("3")),
		Color: color,
	}
}
