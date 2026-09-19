package ui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"vegadesk/internal/ui/theme"
)

// FitColumns adapts cols to totalWidth, so a table stays inside a narrow
// terminal (80x24 over plain ssh is the canonical case) instead of
// overflowing and wrapping, while still using the extra room a wide
// terminal offers instead of leaving it blank.
//
// With room to spare, the surplus goes entirely to the last column (usually
// the widest free-text field -- description, message, command), matching a
// btop-style layout where one column visibly absorbs the slack. Short on
// room, every column shrinks together (each floored at 4, so nothing
// vanishes to zero-width) rather than only the last one -- a table's
// earlier, fixed-looking columns can individually already exceed a small
// terminal's width (e.g. systemd unit names), so shrinking only the last
// column wouldn't be enough to stop wrapping.
func FitColumns(cols []table.Column, totalWidth int) []table.Column {
	out := append([]table.Column(nil), cols...)
	if len(out) == 0 {
		return out
	}
	const minCol = 4
	slack := 3 * len(out) // cell padding + inter-column spacing, per column
	budget := totalWidth - slack
	if budget < minCol*len(out) {
		budget = minCol * len(out)
	}

	sum := 0
	for _, c := range out {
		sum += c.Width
	}
	if sum <= budget {
		out[len(out)-1].Width += budget - sum
		return out
	}

	scale := float64(budget) / float64(sum)
	used := 0
	for i := range out {
		w := int(float64(out[i].Width) * scale)
		if w < minCol {
			w = minCol
		}
		out[i].Width = w
		used += w
	}
	if diff := budget - used; diff > 0 {
		out[len(out)-1].Width += diff
	}
	return out
}

// TableStyles builds bubbles/table styling from the shared theme, so every
// list-based feature (processes, storage, services, containers, VMs, users,
// logs) looks consistent instead of each re-deriving colors.
func TableStyles(s theme.Styles) table.Styles {
	return table.Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(s.Pal.Cyan).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(s.Pal.Border).
			Padding(0, 1),
		Cell: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(s.Pal.Text),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(s.Pal.Background).
			Background(s.Pal.Accent).
			Padding(0, 1),
	}
}
