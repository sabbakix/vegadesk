// Package logs browses the most recent systemd journal entries, with a
// hand-off to `journalctl -f` for true live following.
package logs

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sabbakix/vegadesk/internal/collectors/logs"
	"github.com/sabbakix/vegadesk/internal/ui"
	"github.com/sabbakix/vegadesk/internal/ui/theme"
)

const entryCount = 300

// Model is the Logs section.
type Model struct {
	styles theme.Styles
	table  table.Model
	status string
	err    error
}

func baseColumns() []table.Column {
	return []table.Column{
		{Title: "TIME", Width: 20},
		{Title: "PRI", Width: 4},
		{Title: "SOURCE", Width: 24},
		{Title: "MESSAGE", Width: 70},
	}
}

// New builds a Logs section.
func New(styles theme.Styles) *Model {
	t := table.New(table.WithColumns(baseColumns()), table.WithFocused(true), table.WithHeight(20))
	t.SetStyles(ui.TableStyles(styles))
	return &Model{styles: styles, table: t}
}

func (m *Model) Title() string { return "Logs" }

func (m *Model) Init() tea.Cmd { return sampleCmd() }

type sampleMsg struct {
	entries []logs.Entry
	err     error
}

func sampleCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		entries, err := logs.Collect(ctx, entryCount)
		return sampleMsg{entries: entries, err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.table.SetHeight(msg.Height - 4)
		m.table.SetColumns(ui.FitColumns(baseColumns(), msg.Width))
		return m, nil

	case sampleMsg:
		m.applySample(msg)
		return m, nil

	case ui.SuspendDoneMsg:
		m.status = "log follow closed"
		return m, sampleCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, sampleCmd()
		case "f":
			return m, ui.Suspend("journalctl -f", []string{"journalctl", "-f", "-n", "200"})
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) applySample(s sampleMsg) {
	m.err = s.err
	if s.err != nil {
		return
	}
	rows := make([]table.Row, len(s.entries))
	for i, e := range s.entries {
		rows[i] = table.Row{
			e.Time.Format("Jan 02 15:04:05"),
			e.Priority,
			e.Source(),
			e.Message,
		}
	}
	m.table.SetRows(rows)
	m.table.GotoBottom()
}

func (m *Model) View() string {
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}
	view := m.table.View() + "\n" +
		m.styles.Muted.Render(fmt.Sprintf("showing last %d entries  r: refresh  f: follow live (ctrl-c to return)", entryCount))
	if m.status != "" {
		view += "\n" + m.styles.Muted.Render(m.status)
	}
	return view
}
