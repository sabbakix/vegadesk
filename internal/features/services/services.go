// Package services lists systemd service units with start/stop/restart/
// enable/disable actions and a live log tail.
package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"vegadesk/internal/collectors/service"
	"vegadesk/internal/privilege"
	"vegadesk/internal/ui"
	"vegadesk/internal/ui/theme"
)

const refreshInterval = 5 * time.Second

// tickMsg carries the generation it was scheduled under; see processes.go's
// tickMsg for why.
type tickMsg struct{ gen int }

type sampleMsg struct {
	units []service.Unit
	err   error
}

// Model is the Services section.
type Model struct {
	styles theme.Styles
	priv   privilege.Status
	table  table.Model
	units  []service.Unit
	gen    int

	confirm       *ui.Confirm
	confirmTarget service.Unit
	status        string
	err           error
}

func baseColumns() []table.Column {
	return []table.Column{
		{Title: "UNIT", Width: 34},
		{Title: "ACTIVE", Width: 9},
		{Title: "SUB", Width: 10},
		{Title: "ENABLED", Width: 9},
		{Title: "DESCRIPTION", Width: 40},
	}
}

// New builds a Services section.
func New(styles theme.Styles, priv privilege.Status) *Model {
	t := table.New(table.WithColumns(baseColumns()), table.WithFocused(true), table.WithHeight(18))
	t.SetStyles(ui.TableStyles(styles))
	return &Model{styles: styles, priv: priv, table: t}
}

func (m *Model) Title() string { return "Services" }

// ModalActive reports whether an action confirmation is on screen.
func (m *Model) ModalActive() bool { return m.confirm.Active() }

func (m *Model) Init() tea.Cmd {
	m.gen++
	return tea.Batch(sampleCmd(), tickCmd(m.gen))
}

func tickCmd(gen int) tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
}

func sampleCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		units, err := service.Collect(ctx)
		return sampleMsg{units: units, err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// See processes.go's Update for why tick/sample are handled before the
	// confirm-dialog gate: an open dialog must not stall the refresh loop.
	switch msg := msg.(type) {
	case tickMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		return m, tea.Batch(sampleCmd(), tickCmd(m.gen))
	case sampleMsg:
		m.applySample(msg)
		return m, nil
	}

	if m.confirm.Active() {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.table.SetHeight(msg.Height - 6)
		m.table.SetColumns(ui.FitColumns(baseColumns(), msg.Width))
		return m, nil

	case ui.ConfirmResultMsg:
		return m, m.handleConfirm(msg)

	case ui.SuspendDoneMsg:
		m.status = msg.Label + " closed"
		return m, sampleCmd()

	case ui.PrivilegedDoneMsg:
		if msg.Err != nil {
			m.status = fmt.Sprintf("%s failed: %v", msg.Label, msg.Err)
		} else {
			m.status = msg.Label + ": done"
		}
		return m, sampleCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, sampleCmd()
		case "s":
			return m, m.action("start")
		case "S":
			return m, m.confirmAction("stop")
		case "R":
			return m, m.confirmAction("restart")
		case "e":
			return m, m.action("enable")
		case "d":
			return m, m.confirmAction("disable")
		case "l":
			return m, m.followLogs()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) selected() (service.Unit, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.units) {
		return service.Unit{}, false
	}
	return m.units[idx], true
}

// action runs systemctl <verb> <unit> immediately (start/enable are
// low-risk: they don't interrupt anything already running).
func (m *Model) action(verb string) tea.Cmd {
	u, ok := m.selected()
	if !ok {
		return nil
	}
	if !m.priv.CanAttemptWrite() {
		m.status = m.priv.ReadOnlyReason()
		return nil
	}
	return ui.RunPrivileged(m.priv, verb+" "+u.Name, []string{"systemctl", verb, u.Name})
}

// confirmAction gates verbs that interrupt a running service (stop/restart)
// or remove it from boot (disable) behind a confirmation.
func (m *Model) confirmAction(verb string) tea.Cmd {
	u, ok := m.selected()
	if !ok {
		return nil
	}
	if !m.priv.CanAttemptWrite() {
		m.status = m.priv.ReadOnlyReason()
		return nil
	}
	m.confirmTarget = u
	m.confirm = ui.NewConfirm(verb,
		fmt.Sprintf("%s %s", verb, u.Name),
		u.Description,
		"",
	)
	return nil
}

// handleConfirm uses m.confirmTarget, captured when the dialog opened, not
// the current cursor -- the table keeps refreshing while a dialog is open,
// so the cursor can point at a different unit by the time the user answers.
func (m *Model) handleConfirm(res ui.ConfirmResultMsg) tea.Cmd {
	if !res.Confirmed {
		m.status = "cancelled"
		return nil
	}
	u := m.confirmTarget
	return ui.RunPrivileged(m.priv, res.ID+" "+u.Name, []string{"systemctl", res.ID, u.Name})
}

func (m *Model) followLogs() tea.Cmd {
	u, ok := m.selected()
	if !ok {
		return nil
	}
	return ui.Suspend("journalctl -u "+u.Name, []string{"journalctl", "-u", u.Name, "-f", "-n", "200"})
}

func (m *Model) applySample(s sampleMsg) {
	m.err = s.err
	if s.err != nil {
		return
	}
	units := append([]service.Unit(nil), s.units...)
	sort.SliceStable(units, func(i, j int) bool { return units[i].Name < units[j].Name })
	m.units = units

	rows := make([]table.Row, len(units))
	for i, u := range units {
		rows[i] = table.Row{u.Name, u.Active, u.Sub, u.Enabled, u.Description}
	}
	m.table.SetRows(rows)
}

func (m *Model) View() string {
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}
	view := m.table.View() + "\n" +
		m.styles.Muted.Render("s: start  S: stop  R: restart  e: enable  d: disable  l: follow logs  r: refresh")
	if m.status != "" {
		view += "\n" + m.styles.Muted.Render(m.status)
	}
	if m.confirm.Active() {
		view += "\n\n" + m.confirm.View(m.styles, 60)
	}
	return view
}
