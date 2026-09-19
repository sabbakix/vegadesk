package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"vegadesk/internal/ui/theme"
)

// eighths gives sub-character horizontal resolution for bar fills, the same
// technique btop/htop-style meters use: a full block per whole unit, one of
// seven partial-width glyphs for the fractional remainder.
var eighths = []rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

// BarString renders a width-wide horizontal bar filled to frac (0..1).
func BarString(width int, frac float64) string {
	if width <= 0 {
		return ""
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	totalEighths := int(math.Round(frac * float64(width) * 8))
	full := totalEighths / 8
	rem := totalEighths % 8
	if full >= width {
		return strings.Repeat("█", width)
	}
	var b strings.Builder
	b.WriteString(strings.Repeat("█", full))
	b.WriteRune(eighths[rem])
	if pad := width - full - 1; pad > 0 {
		b.WriteString(strings.Repeat(" ", pad))
	}
	return b.String()
}

// Gauge renders "label [=====     ]  42.3%" within the given total width,
// with the bar colored by theme.Styles.LevelColor(pct). Unlike a naive
// fixed-width format string, this actually fits width down to ~14 columns
// (multi-column layouts like Overview's per-core gauges pack many of these
// into a narrow terminal, where a hardcoded 10-char label would silently
// blow the budget and wrap the line).
func Gauge(s theme.Styles, label string, pct float64, width int) string {
	pctStr := fmt.Sprintf("%5.1f%%", pct) // fixed 6 chars, e.g. " 47.2%"
	const punctuation = 4                 // " [" + "] "

	labelWidth := 10
	if avail := width - punctuation - len(pctStr) - 1; avail < labelWidth {
		labelWidth = avail
	}
	if labelWidth < 1 {
		labelWidth = 1
	}
	label = truncate(label, labelWidth)

	barWidth := width - labelWidth - punctuation - len(pctStr)
	if barWidth < 1 {
		barWidth = 1
	}

	color := s.LevelColor(pct)
	bar := lipgloss.NewStyle().Foreground(color).Render(BarString(barWidth, pct/100))
	return fmt.Sprintf("%-*s [%s] %s", labelWidth, label, bar, pctStr)
}

// truncate clips s to at most n runes, replacing the last one with an
// ellipsis when it doesn't fit whole.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
