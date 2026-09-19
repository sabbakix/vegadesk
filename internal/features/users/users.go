// Package users lists local accounts with lock/unlock/delete actions.
// Creating new accounts needs a multi-field form (username, shell, home,
// initial groups) that's out of scope for this pass -- see the plan's
// storage scope trim for the same reasoning applied here: ship the
// read/manage path now, add the creation wizard later.
package users

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"vegadesk/internal/collectors/accounts"
	"vegadesk/internal/privilege"
	"vegadesk/internal/ui"
	"vegadesk/internal/ui/theme"
)

// Model is the Users section.
type Model struct {
	styles theme.Styles
	priv   privilege.Status
	table  table.Model
	users  []accounts.User

	confirm       *ui.Confirm
	confirmTarget accounts.User
	status        string
	err           error
}

func baseColumns() []table.Column {
	return []table.Column{
		{Title: "NAME", Width: 16},
		{Title: "UID", Width: 6},
		{Title: "HOME", Width: 20},
		{Title: "SHELL", Width: 16},
		{Title: "GROUPS", Width: 30},
	}
}

// New builds a Users section.
func New(styles theme.Styles, priv privilege.Status) *Model {
	t := table.New(table.WithColumns(baseColumns()), table.WithFocused(true), table.WithHeight(14))
	t.SetStyles(ui.TableStyles(styles))
	return &Model{styles: styles, priv: priv, table: t}
}

func (m *Model) Title() string { return "Users" }

// ModalActive reports whether an action confirmation is on screen.
func (m *Model) ModalActive() bool { return m.confirm.Active() }

func (m *Model) Init() tea.Cmd { return sampleCmd() }

type sampleMsg struct {
	users []accounts.User
	err   error
}

func sampleCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		users, err := accounts.Collect(ctx)
		return sampleMsg{users: users, err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// See features/processes's Update for the same reordering and why: a
	// sampleMsg racing an open confirm dialog must still be applied.
	if s, ok := msg.(sampleMsg); ok {
		m.applySample(s)
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
		case "l":
			return m, m.act("lock", []string{"usermod", "-L"})
		case "u":
			return m, m.act("unlock", []string{"usermod", "-U"})
		case "d":
			return m, m.startDelete()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) selected() (accounts.User, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.users) {
		return accounts.User{}, false
	}
	return m.users[idx], true
}

func (m *Model) act(label string, argvPrefix []string) tea.Cmd {
	u, ok := m.selected()
	if !ok {
		return nil
	}
	if !m.priv.CanAttemptWrite() {
		m.status = m.priv.ReadOnlyReason()
		return nil
	}
	return ui.RunPrivileged(m.priv, label+" "+u.Name, append(argvPrefix, u.Name))
}

func (m *Model) startDelete() tea.Cmd {
	u, ok := m.selected()
	if !ok {
		return nil
	}
	if !m.priv.CanAttemptWrite() {
		m.status = m.priv.ReadOnlyReason()
		return nil
	}
	m.confirmTarget = u
	m.confirm = ui.NewConfirm("delete",
		"Delete user "+u.Name,
		fmt.Sprintf("removes the account %s (uid %d). Home directory is kept.", u.Name, u.UID),
		u.Name,
	)
	return nil
}

// handleConfirm uses m.confirmTarget, captured when the dialog opened, not
// the current cursor -- the table can refresh while a dialog is open, so
// the cursor may point at a different user by the time this resolves.
func (m *Model) handleConfirm(res ui.ConfirmResultMsg) tea.Cmd {
	if !res.Confirmed {
		m.status = "cancelled"
		return nil
	}
	u := m.confirmTarget
	return ui.RunPrivileged(m.priv, "delete "+u.Name, []string{"userdel", u.Name})
}

func (m *Model) applySample(s sampleMsg) {
	m.err = s.err
	if s.err != nil {
		return
	}
	var humans []accounts.User
	for _, u := range s.users {
		if u.IsHuman() {
			humans = append(humans, u)
		}
	}
	m.users = humans

	rows := make([]table.Row, len(humans))
	for i, u := range humans {
		rows[i] = table.Row{u.Name, fmt.Sprintf("%d", u.UID), u.Home, u.Shell, strings.Join(u.Groups, ",")}
	}
	m.table.SetRows(rows)
}

func (m *Model) View() string {
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}
	view := m.table.View() + "\n" +
		m.styles.Muted.Render("l: lock  u: unlock  d: delete  r: refresh  (system/service accounts hidden)")
	if m.status != "" {
		view += "\n" + m.styles.Muted.Render(m.status)
	}
	if m.confirm.Active() {
		view += "\n\n" + m.confirm.View(m.styles, 60)
	}
	return view
}
