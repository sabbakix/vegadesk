// Package theme defines vegadesk's color palette and shared styles, btop-inspired:
// dark background, high-contrast accent colors keyed to meaning (green=ok, yellow=warn, red=crit).
package theme

import "github.com/charmbracelet/lipgloss"

// Palette holds the resolved colors for the current color profile.
type Palette struct {
	Background lipgloss.Color
	Surface    lipgloss.Color
	Border     lipgloss.Color
	Text       lipgloss.Color
	Muted      lipgloss.Color
	Accent     lipgloss.Color // primary highlight (selection, active tab)
	Green      lipgloss.Color
	Yellow     lipgloss.Color
	Red        lipgloss.Color
	Cyan       lipgloss.Color
	Magenta    lipgloss.Color
}

// Default is the btop-inspired dark palette. Truecolor hexes degrade automatically
// to the nearest 256/16 color via lipgloss's active color profile.
var Default = Palette{
	Background: lipgloss.Color("#0d1117"),
	Surface:    lipgloss.Color("#161b22"),
	Border:     lipgloss.Color("#30363d"),
	Text:       lipgloss.Color("#c9d1d9"),
	Muted:      lipgloss.Color("#6e7681"),
	Accent:     lipgloss.Color("#58a6ff"),
	Green:      lipgloss.Color("#3fb950"),
	Yellow:     lipgloss.Color("#d29922"),
	Red:        lipgloss.Color("#f85149"),
	Cyan:       lipgloss.Color("#39c5cf"),
	Magenta:    lipgloss.Color("#bc8cff"),
}

// Styles bundles lipgloss.Style values built from a Palette so features don't
// re-derive them. Built once at startup via New.
type Styles struct {
	Pal Palette

	App        lipgloss.Style
	Sidebar    lipgloss.Style
	SidebarSel lipgloss.Style
	SidebarDim lipgloss.Style
	Content    lipgloss.Style
	Header     lipgloss.Style
	Footer     lipgloss.Style
	HelpKey    lipgloss.Style
	HelpDesc   lipgloss.Style
	Border     lipgloss.Style
	Title      lipgloss.Style
	Muted      lipgloss.Style
	Banner     lipgloss.Style // "not available" / read-only notices
	Danger     lipgloss.Style
	OK         lipgloss.Style
	Warn       lipgloss.Style
}

// New builds Styles from a Palette. Call once at startup after resolving the
// color profile (see cmd/vegadesk for the --color flag handling).
func New(p Palette) Styles {
	return Styles{
		Pal: p,

		App: lipgloss.NewStyle().Background(p.Background).Foreground(p.Text),

		Sidebar: lipgloss.NewStyle().
			Foreground(p.Text).
			Padding(0, 1),

		SidebarSel: lipgloss.NewStyle().
			Foreground(p.Background).
			Background(p.Accent).
			Bold(true).
			Padding(0, 1),

		SidebarDim: lipgloss.NewStyle().
			Foreground(p.Muted).
			Padding(0, 1),

		Content: lipgloss.NewStyle().Padding(0, 1),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(p.Background).
			Background(p.Accent).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(p.Muted).
			Padding(0, 1),

		HelpKey:  lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		HelpDesc: lipgloss.NewStyle().Foreground(p.Muted),

		Border: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(p.Border),

		Title: lipgloss.NewStyle().Bold(true).Foreground(p.Cyan),

		Muted: lipgloss.NewStyle().Foreground(p.Muted),

		Banner: lipgloss.NewStyle().
			Foreground(p.Background).
			Background(p.Yellow).
			Bold(true).
			Padding(0, 1),

		Danger: lipgloss.NewStyle().Foreground(p.Red).Bold(true),
		OK:     lipgloss.NewStyle().Foreground(p.Green),
		Warn:   lipgloss.NewStyle().Foreground(p.Yellow),
	}
}

// LevelColor picks Green/Yellow/Red for a 0-100 utilization-style value,
// the common case for gauges and sparklines throughout the app.
func (s Styles) LevelColor(pct float64) lipgloss.Color {
	switch {
	case pct >= 90:
		return s.Pal.Red
	case pct >= 70:
		return s.Pal.Yellow
	default:
		return s.Pal.Green
	}
}
