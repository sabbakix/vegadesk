// Package storage shows block-device/partition topology (lsblk), active
// mounts, and fstab entries, with mount/unmount actions. Partition/
// filesystem creation (mkfs, LVM, btrfs, zpool management) is out of scope
// for this pass -- see the plan's storage scope trim: those are destructive
// and untestable on the dev machine this was built against, so v1 stays
// read-only topology plus mount/unmount.
package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"vegadesk/internal/collectors/blockdev"
	"vegadesk/internal/collectors/disk"
	"vegadesk/internal/privilege"
	"vegadesk/internal/ui"
	"vegadesk/internal/ui/theme"
)

const refreshInterval = 5 * time.Second

// tickMsg carries the generation it was scheduled under; see processes.go's
// tickMsg for why (a stale tick from a previous Init() must not spawn a
// second overlapping refresh loop).
type tickMsg struct{ gen int }

type sampleMsg struct {
	rows []row // built in sampleCmd's goroutine, including per-mount usage%
	err  error
}

type row struct {
	depth      int
	name       string
	sizeBytes  uint64
	devType    string
	fsType     string
	mountpoint string
	usagePct   float64
	hasUsage   bool
	fstabOnly  bool // in /etc/fstab but not currently mounted
}

// Model is the Storage section.
type Model struct {
	styles theme.Styles
	priv   privilege.Status
	table  table.Model
	rows   []row
	gen    int

	confirm       *ui.Confirm
	confirmTarget row // captured when the dialog opens; see handleConfirm
	status        string
	err           error
}

func baseColumns() []table.Column {
	return []table.Column{
		{Title: "NAME", Width: 20},
		{Title: "SIZE", Width: 10},
		{Title: "TYPE", Width: 8},
		{Title: "FSTYPE", Width: 10},
		{Title: "MOUNTPOINT", Width: 24},
		{Title: "USE%", Width: 8},
	}
}

// New builds a Storage section.
func New(styles theme.Styles, priv privilege.Status) *Model {
	t := table.New(table.WithColumns(baseColumns()), table.WithFocused(true), table.WithHeight(15))
	t.SetStyles(ui.TableStyles(styles))
	return &Model{styles: styles, priv: priv, table: t}
}

func (m *Model) Title() string { return "Storage" }

// ModalActive reports whether a mount/unmount confirmation is on screen.
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
		var s sampleMsg
		var devices []blockdev.Device
		var err error
		if devices, err = blockdev.Collect(ctx); err != nil {
			return sampleMsg{err: err}
		}
		mounts, err := blockdev.CollectMounts(ctx)
		if err != nil {
			return sampleMsg{err: err}
		}
		fstab, err := blockdev.CollectFstab(ctx)
		if err != nil {
			return sampleMsg{err: err}
		}

		var rows []row
		flatten(devices, 0, &rows)
		// statfs each mount here, in the background goroutine: a hung
		// network mount must not block the UI thread, which is what
		// calling this from applySample (on Update's goroutine) would do.
		for i := range rows {
			if rows[i].mountpoint != "" && rows[i].mountpoint != "[SWAP]" {
				if u, err := disk.UsagePath(rows[i].mountpoint); err == nil {
					rows[i].usagePct = u.UsedPercent()
					rows[i].hasUsage = true
				}
			}
		}

		mounted := make(map[string]bool, len(mounts))
		for _, mnt := range mounts {
			mounted[mnt.Mountpoint] = true
		}
		for _, fe := range fstab {
			if fe.Mountpoint == "none" || fe.Mountpoint == "swap" || mounted[fe.Mountpoint] {
				continue
			}
			rows = append(rows, row{
				name: fe.Device, fsType: fe.FSType, mountpoint: fe.Mountpoint, fstabOnly: true,
			})
		}

		s.rows = rows
		return s
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
		case "u":
			return m, m.startUnmount()
		case "m":
			return m, m.startMount()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) selected() (row, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return row{}, false
	}
	return m.rows[idx], true
}

func (m *Model) startUnmount() tea.Cmd {
	r, ok := m.selected()
	if !ok || r.mountpoint == "" || r.fstabOnly {
		m.status = "select a mounted row to unmount"
		return nil
	}
	if !m.priv.CanAttemptWrite() {
		m.status = m.priv.ReadOnlyReason()
		return nil
	}
	m.confirmTarget = r
	m.confirm = ui.NewConfirm("umount",
		"Unmount "+r.mountpoint,
		fmt.Sprintf("%s (%s) will be unmounted from %s.", r.name, r.fsType, r.mountpoint),
		r.mountpoint,
	)
	return nil
}

func (m *Model) startMount() tea.Cmd {
	r, ok := m.selected()
	if !ok || !r.fstabOnly {
		m.status = "select an unmounted /etc/fstab entry to mount"
		return nil
	}
	if !m.priv.CanAttemptWrite() {
		m.status = m.priv.ReadOnlyReason()
		return nil
	}
	m.confirmTarget = r
	m.confirm = ui.NewConfirm("mount",
		"Mount "+r.mountpoint,
		fmt.Sprintf("mounts %s per its /etc/fstab entry.", r.mountpoint),
		"",
	)
	return nil
}

// handleConfirm uses m.confirmTarget -- captured when the dialog was opened
// -- rather than re-reading the cursor: the table refreshes live even while
// a dialog is open (see Update's tick/sample reordering), so by the time
// the user answers, the cursor may point at a different row entirely.
func (m *Model) handleConfirm(res ui.ConfirmResultMsg) tea.Cmd {
	if !res.Confirmed {
		m.status = "cancelled"
		return nil
	}
	r := m.confirmTarget
	switch res.ID {
	case "umount":
		return ui.RunPrivileged(m.priv, "umount "+r.mountpoint, []string{"umount", r.mountpoint})
	case "mount":
		return ui.RunPrivileged(m.priv, "mount "+r.mountpoint, []string{"mount", r.mountpoint})
	}
	return nil
}

func (m *Model) applySample(s sampleMsg) {
	m.err = s.err
	if s.err != nil {
		return
	}
	m.rows = s.rows
	m.rebuildTable()
}

func flatten(devs []blockdev.Device, depth int, out *[]row) {
	for _, d := range devs {
		if depth == 0 && d.Type == "loop" {
			// loop-mounted squashfs images (snap packages) aren't
			// manageable storage; they're pure noise here -- often dozens
			// of rows burying the two or three real disks.
			continue
		}
		*out = append(*out, row{
			depth:      depth,
			name:       d.Name,
			sizeBytes:  d.Bytes(),
			devType:    d.Type,
			fsType:     d.FSType,
			mountpoint: d.Mountpoint,
		})
		if len(d.Children) > 0 {
			flatten(d.Children, depth+1, out)
		}
	}
}

func (m *Model) rebuildTable() {
	trows := make([]table.Row, len(m.rows))
	for i, r := range m.rows {
		name := strings.Repeat("  ", r.depth) + r.name
		size := ""
		if r.sizeBytes > 0 {
			size = ui.FormatBytes(float64(r.sizeBytes))
		}
		typ := r.devType
		mount := r.mountpoint
		if r.fstabOnly {
			typ = "fstab"
			mount += " (not mounted)"
		}
		use := ""
		if r.hasUsage {
			use = fmt.Sprintf("%.0f%%", r.usagePct)
		}
		trows[i] = table.Row{name, size, typ, r.fsType, mount, use}
	}
	m.table.SetRows(trows)
}

func (m *Model) View() string {
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}
	view := m.table.View() + "\n" +
		m.styles.Muted.Render("r: refresh  u: unmount selected  m: mount selected fstab entry")
	if m.status != "" {
		view += "\n" + m.styles.Muted.Render(m.status)
	}
	if m.confirm.Active() {
		view += "\n\n" + m.confirm.View(m.styles, 60)
	}
	return view
}
