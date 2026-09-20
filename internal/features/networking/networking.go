// Package networking shows interface addresses/state (via `ip -j addr`) and
// live throughput (via /proc/net/dev, the same collector Overview uses).
// This is a read-only view in v1 -- editing connections is NetworkManager/
// systemd-networkd/netplan-specific enough that a safe, distro-agnostic
// write path is a separate, later effort.
package networking

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sabbakix/vegadesk/internal/collectors/net"
	"github.com/sabbakix/vegadesk/internal/collectors/netif"
	"github.com/sabbakix/vegadesk/internal/ui"
	"github.com/sabbakix/vegadesk/internal/ui/theme"
)

const refreshInterval = 2 * time.Second

// tickMsg carries the generation it was scheduled under, so revisiting this
// tab doesn't stack a second overlapping refresh loop on top of one from a
// previous visit; see features/processes's tickMsg for the full rationale.
type tickMsg struct{ gen int }

type sampleMsg struct {
	ifaces []netif.Iface
	stats  []net.IfaceStats
	err    error
}

type row struct {
	name  string
	state string
	mac   string
	ipv4  string
	rxBps float64
	txBps float64
}

// Model is the Networking section.
type Model struct {
	styles theme.Styles
	table  table.Model

	prevStats map[string]net.IfaceStats
	lastTime  time.Time
	gen       int
	err       error
}

func baseColumns() []table.Column {
	return []table.Column{
		{Title: "IFACE", Width: 12},
		{Title: "STATE", Width: 8},
		{Title: "MAC", Width: 18},
		{Title: "IPV4", Width: 16},
		{Title: "RX/s", Width: 10},
		{Title: "TX/s", Width: 10},
	}
}

// New builds a Networking section.
func New(styles theme.Styles) *Model {
	t := table.New(table.WithColumns(baseColumns()), table.WithFocused(true), table.WithHeight(14))
	t.SetStyles(ui.TableStyles(styles))
	return &Model{styles: styles, table: t, prevStats: map[string]net.IfaceStats{}}
}

func (m *Model) Title() string { return "Networking" }

func (m *Model) Init() tea.Cmd {
	m.gen++
	return tea.Batch(sampleCmd(), tickCmd(m.gen))
}

func tickCmd(gen int) tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
}

func sampleCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var s sampleMsg
		var err error
		if s.ifaces, err = netif.Collect(ctx); err != nil {
			s.err = err
		}
		if s.stats, err = net.Collect(ctx); err != nil && s.err == nil {
			s.err = err
		}
		return s
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.table.SetHeight(msg.Height - 4)
		m.table.SetColumns(ui.FitColumns(baseColumns(), msg.Width))
		return m, nil

	case tickMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		return m, tea.Batch(sampleCmd(), tickCmd(m.gen))

	case sampleMsg:
		m.applySample(msg)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "r" {
			return m, sampleCmd()
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
	now := time.Now()
	elapsed := now.Sub(m.lastTime).Seconds()

	statsByName := make(map[string]net.IfaceStats, len(s.stats))
	for _, st := range s.stats {
		statsByName[st.Name] = st
	}

	rows := make([]row, 0, len(s.ifaces))
	for _, ifc := range s.ifaces {
		r := row{name: ifc.Name, state: ifc.OperState, mac: ifc.Address, ipv4: ifc.IPv4()}
		if cur, ok := statsByName[ifc.Name]; ok {
			if prev, ok := m.prevStats[ifc.Name]; ok && elapsed > 0 {
				r.rxBps = net.RateBps(prev.RxBytes, cur.RxBytes, elapsed)
				r.txBps = net.RateBps(prev.TxBytes, cur.TxBytes, elapsed)
			}
			m.prevStats[ifc.Name] = cur
		}
		rows = append(rows, r)
	}
	m.lastTime = now

	trows := make([]table.Row, len(rows))
	for i, r := range rows {
		trows[i] = table.Row{r.name, r.state, r.mac, r.ipv4, ui.FormatBytes(r.rxBps) + "/s", ui.FormatBytes(r.txBps) + "/s"}
	}
	m.table.SetRows(trows)
}

func (m *Model) View() string {
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}
	return m.table.View() + "\n" + m.styles.Muted.Render("r: refresh")
}
