// Package containers lists Docker or Podman containers with start/stop/rm,
// log-follow, and interactive exec.
package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"vegadesk/internal/collectors/containers"
	"vegadesk/internal/ui"
	"vegadesk/internal/ui/theme"
)

const refreshInterval = 5 * time.Second

// tickMsg carries the generation it was scheduled under; see
// features/processes's tickMsg for why.
type tickMsg struct{ gen int }

type sampleMsg struct {
	items []containers.Container
	err   error
}

// Model is the Containers section.
type Model struct {
	styles theme.Styles
	rt     containers.Runtime
	table  table.Model
	items  []containers.Container
	gen    int

	confirm       *ui.Confirm
	confirmTarget containers.Container
	status        string
	err           error
}

func baseColumns() []table.Column {
	return []table.Column{
		{Title: "ID", Width: 14},
		{Title: "NAME", Width: 20},
		{Title: "IMAGE", Width: 30},
		{Title: "STATE", Width: 10},
		{Title: "STATUS", Width: 26},
	}
}

// New builds a Containers section for the detected runtime. rt is nil when
// neither docker nor podman was found; the section is gated unavailable by
// the sidebar in that case and New is not called.
func New(styles theme.Styles, rt containers.Runtime) *Model {
	t := table.New(table.WithColumns(baseColumns()), table.WithFocused(true), table.WithHeight(16))
	t.SetStyles(ui.TableStyles(styles))
	return &Model{styles: styles, rt: rt, table: t}
}

func (m *Model) Title() string { return "Containers (" + m.rt.Name() + ")" }

// ModalActive reports whether an action confirmation is on screen.
func (m *Model) ModalActive() bool { return m.confirm.Active() }

func (m *Model) Init() tea.Cmd {
	m.gen++
	return tea.Batch(m.sampleCmd(), tickCmd(m.gen))
}

func tickCmd(gen int) tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
}

func (m *Model) sampleCmd() tea.Cmd {
	rt := m.rt
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		items, err := rt.List(ctx)
		return sampleMsg{items: items, err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// See features/processes's Update for why tick/sample are handled
	// before the confirm-dialog gate: an open dialog must not stall the
	// refresh loop.
	switch msg := msg.(type) {
	case tickMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		return m, tea.Batch(m.sampleCmd(), tickCmd(m.gen))
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
		if msg.Err != nil {
			m.status = fmt.Sprintf("%s: %v", msg.Label, msg.Err)
		} else {
			m.status = msg.Label + " closed"
		}
		return m, m.sampleCmd()

	case actionDoneMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s failed: %v", msg.label, msg.err)
		} else {
			m.status = msg.label + ": done"
		}
		return m, m.sampleCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, m.sampleCmd()
		case "s":
			return m, m.runAction("start", m.rt.Start)
		case "S":
			return m, m.confirmAction("stop")
		case "d":
			return m, m.confirmAction("rm")
		case "l":
			return m, m.followLogs()
		case "e":
			return m, m.execShell()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) selected() (containers.Container, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.items) {
		return containers.Container{}, false
	}
	return m.items[idx], true
}

type actionDoneMsg struct {
	label string
	err   error
}

// runAction runs a low-risk verb (start) immediately, without confirmation.
func (m *Model) runAction(label string, fn func(context.Context, string) error) tea.Cmd {
	c, ok := m.selected()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := fn(ctx, c.ID)
		return actionDoneMsg{label: label + " " + shortID(c), err: err}
	}
}

// confirmAction gates verbs that stop a running workload or destroy state
// (stop/rm) behind a confirmation dialog.
func (m *Model) confirmAction(verb string) tea.Cmd {
	c, ok := m.selected()
	if !ok {
		return nil
	}
	m.confirmTarget = c
	m.confirm = ui.NewConfirm(verb,
		fmt.Sprintf("%s %s", verb, shortID(c)), c.Image, "")
	return nil
}

// handleConfirm uses m.confirmTarget, captured when the dialog opened, not
// the current cursor -- the table keeps refreshing while a dialog is open,
// so the cursor can point at a different container by the time the user
// answers.
func (m *Model) handleConfirm(res ui.ConfirmResultMsg) tea.Cmd {
	if !res.Confirmed {
		m.status = "cancelled"
		return nil
	}
	c := m.confirmTarget
	var fn func(context.Context, string) error
	switch res.ID {
	case "stop":
		fn = m.rt.Stop
	case "rm":
		fn = m.rt.Remove
	default:
		return nil
	}
	label := res.ID + " " + shortID(c)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return actionDoneMsg{label: label, err: fn(ctx, c.ID)}
	}
}

func (m *Model) followLogs() tea.Cmd {
	c, ok := m.selected()
	if !ok {
		return nil
	}
	return ui.Suspend("logs "+shortID(c), m.rt.LogsArgv(c.ID))
}

func (m *Model) execShell() tea.Cmd {
	c, ok := m.selected()
	if !ok {
		return nil
	}
	return ui.Suspend("exec "+shortID(c), m.rt.ExecArgv(c.ID, "/bin/sh"))
}

func shortID(c containers.Container) string {
	if c.Name != "" {
		return c.Name
	}
	if len(c.ID) > 12 {
		return c.ID[:12]
	}
	return c.ID
}

func (m *Model) applySample(s sampleMsg) {
	m.err = s.err
	if s.err != nil {
		return
	}
	m.items = s.items
	rows := make([]table.Row, len(s.items))
	for i, c := range s.items {
		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}
		rows[i] = table.Row{id, c.Name, c.Image, c.State, c.Status}
	}
	m.table.SetRows(rows)
}

func (m *Model) View() string {
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}
	view := m.table.View() + "\n" +
		m.styles.Muted.Render("s: start  S: stop  d: remove  l: follow logs  e: exec shell  r: refresh")
	if m.status != "" {
		view += "\n" + m.styles.Muted.Render(m.status)
	}
	if m.confirm.Active() {
		view += "\n\n" + m.confirm.View(m.styles, 60)
	}
	return view
}
