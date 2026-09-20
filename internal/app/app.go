// Package app hosts the root Bubble Tea model: sidebar navigation across
// feature sections, global keybindings, and the footer help/status bar.
// Each section is an independent tea.Model so features stay decoupled from
// each other and from the shell.
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sabbakix/vegadesk/internal/platform"
	"github.com/sabbakix/vegadesk/internal/privilege"
	"github.com/sabbakix/vegadesk/internal/ui/theme"
)

// Section is a feature module pluggable into the sidebar. Update must accept
// and gracefully ignore message types it doesn't recognize, since the root
// model forwards window-size messages to every section regardless of which
// is active.
type Section interface {
	tea.Model
	Title() string
}

// ModalHolder is implemented by sections that can show a blocking dialog
// (a destructive-action confirmation). Root checks this before treating a
// key as global navigation -- see Update's tea.KeyMsg case.
type ModalHolder interface {
	ModalActive() bool
}

type sectionEntry struct {
	jump      string // "1".."9","0"
	name      string
	model     Section
	available bool
	reason    string // shown in place of the section when unavailable
}

// Model is the root Bubble Tea model.
type Model struct {
	styles theme.Styles
	keys   globalKeyMap
	help   help.Model

	caps platform.Capabilities
	priv privilege.Status

	width, height int
	sections      []sectionEntry
	active        int
	showHelp      bool
	status        string // last suspend-process result / transient notice
}

// New builds the root model. Sections are registered by the caller (see
// registerSections in cmd/vegadesk) so app itself has no knowledge of any
// specific feature.
func New(styles theme.Styles, caps platform.Capabilities, priv privilege.Status) *Model {
	h := help.New()
	h.ShowAll = false
	return &Model{
		styles: styles,
		keys:   newGlobalKeys(),
		help:   h,
		caps:   caps,
		priv:   priv,
	}
}

// AddSection registers a section under the sidebar. available/reason let the
// caller apply the platform-capability gate once, in one place, rather than
// every feature re-deriving it.
func (m *Model) AddSection(name string, s Section, available bool, reason string) {
	digits := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}
	jump := ""
	if len(m.sections) < len(digits) {
		jump = digits[len(m.sections)]
	}
	m.sections = append(m.sections, sectionEntry{
		jump:      jump,
		name:      name,
		model:     s,
		available: available,
		reason:    reason,
	})
}

func (m *Model) Init() tea.Cmd {
	if len(m.sections) == 0 {
		return nil
	}
	return m.sections[m.active].model.Init()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Sections must size themselves to the actual content pane (after
		// the sidebar, header, and padding), not the raw terminal size, or
		// their graphs/gauges overflow the pane and wrap ugly. Reuse the
		// same layout math View() uses so the two can never drift apart.
		_, _, _, innerWidth, innerHeight := m.layout()
		contentMsg := tea.WindowSizeMsg{Width: innerWidth, Height: innerHeight}
		var cmds []tea.Cmd
		for i, se := range m.sections {
			updated, cmd := se.model.Update(contentMsg)
			if s, ok := updated.(Section); ok {
				m.sections[i].model = s
			}
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Checked even with a modal open: on a typed-confirmation dialog
		// (e.g. "type the mountpoint to confirm"), every key but esc goes
		// to the text input, so ctrl+c would otherwise be silently
		// swallowed instead of force-quitting -- which reads as a hang.
		if key.Matches(msg, m.keys.ForceQuit) {
			return m, tea.Quit
		}
		// While the active section has a confirmation dialog open, every
		// other key -- including "q", help, and tab/digit navigation --
		// must go to that section instead: switching away would leave the
		// dialog "open" with nothing left routing input to it, silently
		// wedging the section until it's revisited and closed. (A plain
		// y/n dialog does treat "q" as cancel, so quit-via-q still works
		// there once routed through.)
		if !m.activeSectionModalOpen() {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Help):
				m.showHelp = !m.showHelp
				return m, nil
			case key.Matches(msg, m.keys.NextTab):
				return m, m.switchTo(m.nextAvailable(1))
			case key.Matches(msg, m.keys.PrevTab):
				return m, m.switchTo(m.nextAvailable(-1))
			}
			for i, b := range m.keys.JumpKeys {
				if i < len(m.sections) && key.Matches(msg, b) {
					return m, m.switchTo(i)
				}
			}
		}
	}

	if len(m.sections) == 0 {
		return m, nil
	}
	active := &m.sections[m.active]
	updated, cmd := active.model.Update(msg)
	if s, ok := updated.(Section); ok {
		active.model = s
	}
	return m, cmd
}

func (m *Model) activeSectionModalOpen() bool {
	if len(m.sections) == 0 {
		return false
	}
	mh, ok := m.sections[m.active].model.(ModalHolder)
	return ok && mh.ModalActive()
}

func (m *Model) switchTo(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.sections) || idx == m.active {
		return nil
	}
	m.active = idx
	return m.sections[idx].model.Init()
}

// nextAvailable cycles to the immediate neighbor in dir even if it's
// unavailable on this host -- its placeholder explains why, which is more
// honest than silently skipping it.
func (m *Model) nextAvailable(dir int) int {
	if len(m.sections) == 0 {
		return m.active
	}
	return (m.active + dir + len(m.sections)) % len(m.sections)
}

// layout computes the shell's geometry once, shared by View (to actually
// draw it) and Update (to tell sections how much room they have). sidebarW
// and contentW are the two top-level columns; bodyH is the content area's
// height above the footer; innerW/innerH are what a section's own View
// should assume it has to work with (content pane minus header minus the
// content style's padding).
func (m *Model) layout() (sidebarW, contentW, bodyH, innerW, innerH int) {
	sidebarW = 20
	contentW = m.width - sidebarW
	if contentW < 20 {
		contentW = m.width
		sidebarW = 0
	}

	footerH := lipgloss.Height(m.renderFooter())
	bodyH = m.height - footerH
	if bodyH < 1 {
		bodyH = 1
	}

	const headerH = 1 // Header style is always a single unwrapped line
	innerH = bodyH - headerH
	if innerH < 1 {
		innerH = 1
	}
	innerW = contentW - 2 // Content style's Padding(0, 1) on each side
	if innerW < 1 {
		innerW = 1
	}
	return
}

func (m *Model) View() string {
	if m.width == 0 {
		return "starting vegadesk..."
	}

	sidebarWidth, contentWidth, bodyHeight, _, _ := m.layout()
	footer := m.renderFooter()

	var body string
	if sidebarWidth > 0 {
		sidebar := m.renderSidebar(sidebarWidth, bodyHeight)
		content := m.renderContent(contentWidth, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	} else {
		body = m.renderContent(contentWidth, bodyHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

func (m *Model) renderSidebar(width, height int) string {
	var b strings.Builder
	// Padding(0, 1) on the sidebar styles consumes one column each side;
	// truncate so a long section title (e.g. "Containers (docker)") can
	// never wrap the sidebar onto a second line and misalign every row
	// below it.
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	for i, se := range m.sections {
		label := se.name
		if se.jump != "" {
			label = fmt.Sprintf("%s %s", se.jump, se.name)
		}
		label = truncate(label, innerWidth)
		switch {
		case i == m.active:
			b.WriteString(m.styles.SidebarSel.Copy().Width(width).Render(label))
		case !se.available:
			b.WriteString(m.styles.SidebarDim.Copy().Width(width).Render(label))
		default:
			b.WriteString(m.styles.Sidebar.Copy().Width(width).Render(label))
		}
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
}

func (m *Model) renderContent(width, height int) string {
	if len(m.sections) == 0 {
		return ""
	}
	se := m.sections[m.active]
	header := m.styles.Header.Copy().Width(width).Render(se.name)

	var content string
	if !se.available {
		content = m.styles.Banner.Render("not available: "+se.reason) + "\n"
	} else {
		content = se.model.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.styles.Content.Copy().Width(width).Height(height-lipgloss.Height(header)).Render(content),
	)
}

func (m *Model) renderFooter() string {
	if m.showHelp {
		return m.styles.Footer.Copy().Width(m.width).Render(m.help.View(m.keys))
	}
	sudo := "sudo: n/a"
	switch {
	case m.priv.IsRoot:
		sudo = "root"
	case m.priv.Passwordless:
		sudo = "sudo: passwordless"
	case m.priv.HasSudo:
		sudo = "sudo: will prompt"
	default:
		sudo = "read-only (no sudo)"
	}
	line := m.help.View(m.keys) + "  |  " + sudo
	if m.status != "" {
		line += "  |  " + m.status
	}
	return m.styles.Footer.Copy().Width(m.width).Render(line)
}

// truncate clips s to at most width runes, replacing the last one with an
// ellipsis when it doesn't fit.
func truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "…"
}

// SetStatus sets the transient footer status line (e.g. after a suspended
// command returns). Exported for cmd/vegadesk's root Update wiring if needed.
func (m *Model) SetStatus(s string) { m.status = s }
