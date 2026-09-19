package app

import tea "github.com/charmbracelet/bubbletea"

// Placeholder stands in for a section not yet implemented in this build
// phase. It renders a short note instead of crashing or showing a blank
// pane, so the sidebar can list every planned section from day one.
type Placeholder struct {
	title string
	note  string
}

// NewPlaceholder builds a stand-in section, e.g. for a feature phase not
// built yet ("coming in a later phase") independent of host capability.
func NewPlaceholder(title, note string) *Placeholder {
	return &Placeholder{title: title, note: note}
}

func (p *Placeholder) Init() tea.Cmd                           { return nil }
func (p *Placeholder) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return p, nil }
func (p *Placeholder) View() string                            { return p.note }
func (p *Placeholder) Title() string                           { return p.title }
