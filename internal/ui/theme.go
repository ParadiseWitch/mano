package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Column widths of a log row, in terminal cells.
const (
	colIndex    = 3
	colTime     = 5
	colDuration = 6
	colGap      = 1
	rowMargin   = 1

	// fixedWidth is every cell left of the content column, margins included.
	fixedWidth = rowMargin + colIndex + colTime*2 + colDuration + colGap*4
)

// Every colour pins a foreground and a background together, so a block reads
// the same whether the terminal behind it is light or dark.
var (
	fgText   = lipgloss.Color("252")
	fgDim    = lipgloss.Color("244")
	fgBright = lipgloss.Color("255")
	fgInk    = lipgloss.Color("232")

	bgBlock   = lipgloss.Color("236")
	bgSelect  = lipgloss.Color("24")
	bgField   = lipgloss.Color("214")
	bgCrossed = lipgloss.Color("94")
	bgDivider = lipgloss.Color("238")
	bgStatus  = lipgloss.Color("235")
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	dimStyle    = lipgloss.NewStyle().Foreground(fgDim)
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	statusStyle = lipgloss.NewStyle().Background(bgStatus).Foreground(fgDim)
)

// fit truncates to width so lipgloss pads the cell instead of wrapping it onto
// a second line and breaking the row.
func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "")
}

// cell renders one fixed-width colour block.
func cell(text string, width int, align lipgloss.Position, fg, bg lipgloss.Color) string {
	return lipgloss.NewStyle().
		Width(width).
		Align(align).
		Foreground(fg).
		Background(bg).
		Render(fit(text, width))
}

// divider is a full-width colour band. A band of spaces is used rather than
// box-drawing glyphs because U+2500 is double-width in some CJK terminals.
func divider(width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Background(bgDivider).
		Render(strings.Repeat(" ", width))
}

// spread lays left and right text at the two ends of a width-sized line.
func spread(width int, left, right string) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return fit(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// wrap folds v into [0, mod), so a spinner can run past either end.
func wrap(v, mod int) int {
	return ((v % mod) + mod) % mod
}
