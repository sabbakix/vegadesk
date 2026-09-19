// Package vms lists libvirt domains with start/shutdown/destroy and an
// interactive console.
package vms

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"vegadesk/internal/collectors/vm"
	"vegadesk/internal/execx"
	"vegadesk/internal/ui"
	"vegadesk/internal/ui/theme"
)

const refreshInterval = 5 * time.Second

// tickMsg carries the generation it was scheduled under; see
// features/processes's tickMsg for why.
type tickMsg struct{ gen int }

type sampleMsg struct {
	domains []vm.Domain
	err     error
}

type actionDoneMsg struct {
	label string
	err   error
}

// Model is the Virtual Machines section.
type Model struct {
	styles  theme.Styles
	table   table.Model
	domains []vm.Domain
	gen     int

	confirm       *ui.Confirm
	confirmTarget vm.Domain
	status        string
	err           error
}

func baseColumns() []table.Column {
	return []table.Column{
		{Title: "ID", Width: 6},
		{Title: "NAME", Width: 28},
		{Title: "STATE", Width: 16},
	}
}

// New builds a Virtual Machines section.
func New(styles theme.Styles) *Model {
	t := table.New(table.WithColumns(baseColumns()), table.WithFocused(true), table.WithHeight(16))
	t.SetStyles(ui.TableStyles(styles))
	return &Model{styles: styles, table: t}
}

func (m *Model) Title() string { return "Virtual Machines" }

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
		domains, err := vm.Collect(ctx)
		return sampleMsg{domains: domains, err: err}
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
		if msg.Err != nil {
			m.status = fmt.Sprintf("%s: %v", msg.Label, msg.Err)
		} else {
			m.status = msg.Label + " closed"
		}
		return m, sampleCmd()

	case actionDoneMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s failed: %v", msg.label, msg.err)
		} else {
			m.status = msg.label + ": done"
		}
		return m, sampleCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, sampleCmd()
		case "s":
			return m, m.runVerb("start", "start")
		case "S":
			return m, m.confirmVerb("shutdown", "shutdown (graceful)")
		case "D":
			return m, m.confirmVerb("destroy", "destroy (force off)")
		case "c":
			return m, m.console()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) selected() (vm.Domain, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.domains) {
		return vm.Domain{}, false
	}
	return m.domains[idx], true
}

// virsh is run directly, unprivileged, the same way Docker/Podman are: on
// most libvirt setups the invoking user is in the "libvirt" group and needs
// no elevation, so unconditionally prefixing sudo would be wrong more often
// than not. If it fails for lack of permission, the error surfaces as-is.
func (m *Model) runVerb(verb, label string) tea.Cmd {
	d, ok := m.selected()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := execx.Run(ctx, "virsh", verb, d.Name)
		return actionDoneMsg{label: label + " " + d.Name, err: err}
	}
}

func (m *Model) confirmVerb(verb, label string) tea.Cmd {
	d, ok := m.selected()
	if !ok {
		return nil
	}
	m.confirmTarget = d
	m.confirm = ui.NewConfirm(verb, fmt.Sprintf("%s %s", label, d.Name), "state: "+d.State, "")
	return nil
}

// handleConfirm uses m.confirmTarget, captured when the dialog opened, not
// the current cursor -- the table keeps refreshing while a dialog is open,
// so the cursor can point at a different domain by the time the user
// answers.
func (m *Model) handleConfirm(res ui.ConfirmResultMsg) tea.Cmd {
	if !res.Confirmed {
		m.status = "cancelled"
		return nil
	}
	d := m.confirmTarget
	verb := res.ID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := execx.Run(ctx, "virsh", verb, d.Name)
		return actionDoneMsg{label: verb + " " + d.Name, err: err}
	}
}

func (m *Model) console() tea.Cmd {
	d, ok := m.selected()
	if !ok {
		return nil
	}
	return ui.Suspend("console "+d.Name, []string{"virsh", "console", d.Name})
}

func (m *Model) applySample(s sampleMsg) {
	m.err = s.err
	if s.err != nil {
		return
	}
	m.domains = s.domains
	rows := make([]table.Row, len(s.domains))
	for i, d := range s.domains {
		rows[i] = table.Row{d.ID, d.Name, d.State}
	}
	m.table.SetRows(rows)
}

func (m *Model) View() string {
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}
	view := m.table.View() + "\n" +
		m.styles.Muted.Render("s: start  S: shutdown  D: destroy (force)  c: console (ctrl+] to exit)  r: refresh")
	if m.status != "" {
		view += "\n" + m.styles.Muted.Render(m.status)
	}
	if m.confirm.Active() {
		view += "\n\n" + m.confirm.View(m.styles, 60)
	}
	return view
}
