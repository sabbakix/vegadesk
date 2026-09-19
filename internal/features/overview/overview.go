// Package overview is the btop-style landing dashboard: CPU/mem/net history
// graphs, per-core gauges, disk usage, uptime, and load average.
package overview

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"vegadesk/internal/collectors/cpu"
	"vegadesk/internal/collectors/disk"
	"vegadesk/internal/collectors/mem"
	"vegadesk/internal/collectors/net"
	"vegadesk/internal/ui"
	"vegadesk/internal/ui/theme"
)

const (
	refreshInterval = 1 * time.Second
	historyLen      = 240 // 2 wide graph columns/sample -> up to 120 graph cells of history
)

// tickMsg drives the refresh loop; dropped harmlessly by the root app if
// this section isn't the active one when it arrives (see internal/app).
type tickMsg time.Time

// sampleMsg carries a completed collection round back into Update.
type sampleMsg struct {
	cpuSample cpu.Sample
	memInfo   mem.Info
	netStats  []net.IfaceStats
	diskUsage disk.Usage
	err       error
}

// Model is the Overview section.
type Model struct {
	styles theme.Styles

	width, height int

	// previous samples, for delta-based rates (nil until the 2nd tick)
	prevCPU     *cpu.Sample
	prevNet     map[string]net.IfaceStats
	lastSampled time.Time

	cpuHistory  []float64 // aggregate CPU %, most recent last
	coreHistory [][]float64
	memHistory  []float64
	rxHistory   []float64 // aggregate bytes/sec across interfaces
	txHistory   []float64

	curCPU     float64
	curCores   []float64
	curMem     mem.Info
	curDisk    disk.Usage
	curRxBps   float64
	curTxBps   float64
	maxNetSeen float64 // for autoscaling the net graph

	err error
}

// New builds an Overview section.
func New(styles theme.Styles) *Model {
	return &Model{styles: styles, prevNet: map[string]net.IfaceStats{}}
}

func (m *Model) Title() string { return "Overview" }

func (m *Model) Init() tea.Cmd {
	return tea.Batch(sampleCmd(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func sampleCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var m sampleMsg
		var err error
		if m.cpuSample, err = cpu.Collect(ctx); err != nil {
			m.err = err
		}
		if m.memInfo, err = mem.Collect(ctx); err != nil && m.err == nil {
			m.err = err
		}
		if m.netStats, err = net.Collect(ctx); err != nil && m.err == nil {
			m.err = err
		}
		if m.diskUsage, err = disk.UsagePath("/"); err != nil && m.err == nil {
			m.err = err
		}
		return m
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(sampleCmd(), tickCmd())

	case sampleMsg:
		m.applySample(msg)
		return m, nil
	}
	return m, nil
}

func (m *Model) applySample(s sampleMsg) {
	now := time.Now()
	m.err = s.err

	if m.prevCPU != nil {
		m.curCPU = cpu.UsagePercent(m.prevCPU.Total, s.cpuSample.Total)
		m.curCores = make([]float64, len(s.cpuSample.Cores))
		for i, c := range s.cpuSample.Cores {
			if i < len(m.prevCPU.Cores) {
				m.curCores[i] = cpu.UsagePercent(m.prevCPU.Cores[i], c)
			}
		}
		m.cpuHistory = appendCapped(m.cpuHistory, m.curCPU, historyLen)
		if len(m.coreHistory) != len(m.curCores) {
			m.coreHistory = make([][]float64, len(m.curCores))
		}
		for i, v := range m.curCores {
			m.coreHistory[i] = appendCapped(m.coreHistory[i], v, historyLen)
		}
	}
	prevCPU := s.cpuSample
	m.prevCPU = &prevCPU

	m.curMem = s.memInfo
	m.memHistory = appendCapped(m.memHistory, s.memInfo.UsedPercent(), historyLen)

	m.curDisk = s.diskUsage

	if !m.lastSampled.IsZero() {
		elapsed := now.Sub(m.lastSampled).Seconds()
		var rx, tx float64
		for _, ifc := range s.netStats {
			if ifc.Name == "lo" {
				continue
			}
			prev, ok := m.prevNet[ifc.Name]
			if ok {
				rx += net.RateBps(prev.RxBytes, ifc.RxBytes, elapsed)
				tx += net.RateBps(prev.TxBytes, ifc.TxBytes, elapsed)
			}
		}
		m.curRxBps, m.curTxBps = rx, tx
		m.rxHistory = appendCapped(m.rxHistory, rx, historyLen)
		m.txHistory = appendCapped(m.txHistory, tx, historyLen)
		if rx > m.maxNetSeen {
			m.maxNetSeen = rx
		}
		if tx > m.maxNetSeen {
			m.maxNetSeen = tx
		}
	}
	m.lastSampled = now
	for _, ifc := range s.netStats {
		m.prevNet[ifc.Name] = ifc
	}
}

func appendCapped(hist []float64, v float64, cap int) []float64 {
	hist = append(hist, v)
	if len(hist) > cap {
		hist = hist[len(hist)-cap:]
	}
	return hist
}

func (m *Model) View() string {
	if m.prevCPU == nil {
		return m.styles.Muted.Render("sampling...")
	}
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}

	width := m.width - 2
	if width < 20 {
		width = 40
	}
	graphHeight := 5

	var b strings.Builder

	b.WriteString(m.styles.Title.Render(fmt.Sprintf("CPU  %.1f%%", m.curCPU)))
	b.WriteString("\n")
	for _, line := range ui.Graph(m.cpuHistory, width, graphHeight, 100) {
		b.WriteString(colorGraphLine(m.styles, line, m.curCPU))
		b.WriteString("\n")
	}
	b.WriteString(renderCoreGauges(m.styles, m.curCores, width))
	b.WriteString("\n\n")

	b.WriteString(m.styles.Title.Render(fmt.Sprintf("Memory  %.1f%% of %s", m.curMem.UsedPercent(), humanKB(m.curMem.TotalKB))))
	b.WriteString("\n")
	for _, line := range ui.Graph(m.memHistory, width, graphHeight, 100) {
		b.WriteString(colorGraphLine(m.styles, line, m.curMem.UsedPercent()))
		b.WriteString("\n")
	}
	b.WriteString(ui.Gauge(m.styles, "used", m.curMem.UsedPercent(), width))
	b.WriteString("\n")
	if m.curMem.SwapTotalKB > 0 {
		b.WriteString(ui.Gauge(m.styles, "swap", m.curMem.SwapUsedPercent(), width))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(m.styles.Title.Render(fmt.Sprintf("Network  ↓ %s/s  ↑ %s/s", ui.FormatBytes(m.curRxBps), ui.FormatBytes(m.curTxBps))))
	b.WriteString("\n")
	netMax := m.maxNetSeen
	if netMax <= 0 {
		netMax = 1
	}
	for _, line := range ui.Graph(m.rxHistory, width, 3, netMax) {
		b.WriteString(m.styles.OK.Render(line))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(m.styles.Title.Render(fmt.Sprintf("Disk (/)  %.1f%% of %s", m.curDisk.UsedPercent(), ui.FormatBytes(float64(m.curDisk.TotalBytes)))))
	b.WriteString("\n")
	b.WriteString(ui.Gauge(m.styles, "used", m.curDisk.UsedPercent(), width))
	b.WriteString("\n")

	return b.String()
}

func colorGraphLine(s theme.Styles, line string, pct float64) string {
	return lipgloss.NewStyle().Foreground(s.LevelColor(pct)).Render(line)
}

func renderCoreGauges(s theme.Styles, cores []float64, width int) string {
	if len(cores) == 0 {
		return ""
	}
	colWidth := 18
	perRow := width / colWidth
	if perRow < 1 {
		perRow = 1
	}
	var b strings.Builder
	for i, pct := range cores {
		label := fmt.Sprintf("c%d", i)
		b.WriteString(ui.Gauge(s, label, pct, colWidth-1))
		if (i+1)%perRow == 0 {
			b.WriteString("\n")
		} else {
			b.WriteString(" ")
		}
	}
	return b.String()
}

func humanKB(kb uint64) string { return ui.FormatBytes(float64(kb) * 1024) }
