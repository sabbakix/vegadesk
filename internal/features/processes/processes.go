// Package processes lists running processes, sortable by CPU/mem/PID, with
// kill (SIGTERM) and force-kill (SIGKILL) gated behind a confirmation.
package processes

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"vegadesk/internal/collectors/process"
	"vegadesk/internal/ui"
	"vegadesk/internal/ui/theme"
)

const (
	refreshInterval = 2 * time.Second
	// clkTck is USER_HZ, the tick rate /proc/[pid]/stat's utime/stime are
	// reported in. Virtually every Linux distro uses 100 regardless of
	// kernel HZ; there is no portable way to read it from /proc.
	clkTck = 100.0
)

type sortMode int

const (
	sortCPU sortMode = iota
	sortMem
	sortPID
)

// tickMsg carries the generation it was scheduled under, so a tick from a
// refresh loop started by a previous Init() (e.g. before the user tabbed
// away and back) is recognized as stale and dropped instead of spawning a
// second, overlapping refresh loop.
type tickMsg struct{ gen int }

type sampleMsg struct {
	procs []process.Proc
	err   error
}

type prevTimes struct {
	utime, stime uint64
}

// Model is the Processes section.
type Model struct {
	styles theme.Styles
	table  table.Model
	sort   sortMode
	gen    int // refresh-loop generation, see tickMsg

	prev     map[int]prevTimes
	lastTime time.Time
	rows     []row // parallel to table rows, for kill-target lookup by cursor

	userCache map[uint32]string

	confirm *ui.Confirm
	status  string
	err     error
}

type row struct {
	pid     int
	comm    string
	cpuPct  float64
	rssKB   uint64
	user    string
	cmdline string
}

func baseColumns() []table.Column {
	return []table.Column{
		{Title: "PID", Width: 8},
		{Title: "USER", Width: 10},
		{Title: "CPU%", Width: 7},
		{Title: "MEM", Width: 9},
		{Title: "COMMAND", Width: 50},
	}
}

// New builds a Processes section.
func New(styles theme.Styles) *Model {
	t := table.New(table.WithColumns(baseColumns()), table.WithFocused(true), table.WithHeight(20))
	t.SetStyles(ui.TableStyles(styles))
	return &Model{
		styles:    styles,
		table:     t,
		prev:      map[int]prevTimes{},
		userCache: map[uint32]string{},
	}
}

func (m *Model) Title() string { return "Processes" }

// ModalActive reports whether the kill confirmation is on screen, so the
// root app can route every key to us (and onward to the dialog) instead of
// treating digits/tab as section-switch shortcuts while it's open.
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
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		procs, err := process.Collect(ctx)
		return sampleMsg{procs: procs, err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handled before the confirm-dialog gate below so an open dialog can
	// never stall the refresh loop: a tickMsg that arrived while the user
	// was mid-confirmation still needs to re-arm itself, or the section
	// stops refreshing in the background until the dialog closes.
	switch msg := msg.(type) {
	case tickMsg:
		if msg.gen != m.gen {
			return m, nil // stale loop from a previous Init(); let it die
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

	case tea.KeyMsg:
		switch msg.String() {
		case "c":
			m.sort = sortCPU
			m.resort()
			return m, nil
		case "m":
			m.sort = sortMem
			m.resort()
			return m, nil
		case "p":
			m.sort = sortPID
			m.resort()
			return m, nil
		case "x":
			return m, m.startKill(syscall.SIGTERM, false)
		case "X":
			return m, m.startKill(syscall.SIGKILL, true)
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) startKill(sig syscall.Signal, force bool) tea.Cmd {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return nil
	}
	r := m.rows[idx]
	label := "terminate"
	if force {
		label = "force-kill"
	}
	id := fmt.Sprintf("kill:%d:%d", sig, r.pid)
	m.confirm = ui.NewConfirm(id,
		fmt.Sprintf("%s PID %d", label, r.pid),
		fmt.Sprintf("%s\n\nsends %v to this process. This can't be undone.", r.cmdline, sig),
		"",
	)
	return nil
}

func (m *Model) handleConfirm(res ui.ConfirmResultMsg) tea.Cmd {
	if !res.Confirmed {
		m.status = "cancelled"
		return nil
	}
	var sig int
	var pid int
	if _, err := fmt.Sscanf(res.ID, "kill:%d:%d", &sig, &pid); err != nil {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		m.status = fmt.Sprintf("kill %d failed: %v", pid, err)
		return nil
	}
	if err := proc.Signal(syscall.Signal(sig)); err != nil {
		m.status = fmt.Sprintf("kill %d failed: %v (try running as root)", pid, err)
		return nil
	}
	m.status = fmt.Sprintf("sent signal %d to PID %d", sig, pid)
	return sampleCmd()
}

func (m *Model) applySample(s sampleMsg) {
	m.err = s.err
	if s.err != nil {
		return
	}
	now := time.Now()
	elapsed := now.Sub(m.lastTime).Seconds()

	rows := make([]row, 0, len(s.procs))
	nextPrev := make(map[int]prevTimes, len(s.procs))
	for _, p := range s.procs {
		cur := prevTimes{utime: p.UtimeTicks, stime: p.StimeTicks}
		var pct float64
		if prev, ok := m.prev[p.PID]; ok && elapsed > 0 {
			deltaTicks := float64((cur.utime + cur.stime) - (prev.utime + prev.stime))
			if deltaTicks > 0 {
				pct = deltaTicks / clkTck / elapsed * 100
			}
		}
		nextPrev[p.PID] = cur
		rows = append(rows, row{
			pid:     p.PID,
			comm:    p.Comm,
			cpuPct:  pct,
			rssKB:   p.RSSPages * 4, // most Linux systems: 4KB pages; good enough for display
			user:    m.lookupUser(p.UID),
			cmdline: p.Cmdline,
		})
	}
	m.prev = nextPrev
	m.lastTime = now
	m.rows = rows
	m.resort()
}

func (m *Model) lookupUser(uid uint32) string {
	if name, ok := m.userCache[uid]; ok {
		return name
	}
	name := strconv.Itoa(int(uid))
	if u, err := user.LookupId(name); err == nil {
		name = u.Username
	}
	m.userCache[uid] = name
	return name
}

func (m *Model) resort() {
	rows := append([]row(nil), m.rows...)
	switch m.sort {
	case sortCPU:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].cpuPct > rows[j].cpuPct })
	case sortMem:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].rssKB > rows[j].rssKB })
	case sortPID:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].pid < rows[j].pid })
	}
	m.rows = rows

	trows := make([]table.Row, len(rows))
	for i, r := range rows {
		trows[i] = table.Row{
			strconv.Itoa(r.pid),
			r.user,
			fmt.Sprintf("%.1f", r.cpuPct),
			humanKB(r.rssKB),
			r.cmdline,
		}
	}
	m.table.SetRows(trows)
}

func humanKB(kb uint64) string { return ui.FormatBytes(float64(kb) * 1024) }

func (m *Model) View() string {
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}
	view := m.table.View() + "\n" +
		m.styles.Muted.Render("c: sort cpu  m: sort mem  p: sort pid  x: terminate  X: force-kill")
	if m.status != "" {
		view += "\n" + m.styles.Muted.Render(m.status)
	}
	if m.confirm.Active() {
		view += "\n\n" + m.confirm.View(m.styles, 60)
	}
	return view
}
