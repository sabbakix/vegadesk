package ui

import (
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// SuspendDoneMsg is sent once a foreground process started via Suspend exits
// and vegadesk has regained the terminal.
type SuspendDoneMsg struct {
	Label string // what ran, for a status line or error banner
	Err   error
}

// Suspend runs argv in the foreground, handing it the real terminal and
// restoring vegadesk's display (bubbletea forces a full repaint) when it
// exits. Every place that needs an attached TTY -- a sudo password prompt,
// `virsh console`, `docker exec -it`/`podman exec -it`, `journalctl -f`, a
// pager -- goes through this one helper rather than each feature wiring up
// tea.ExecProcess itself.
func Suspend(label string, argv []string) tea.Cmd {
	if len(argv) == 0 {
		return func() tea.Msg { return SuspendDoneMsg{Label: label} }
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return SuspendDoneMsg{Label: label, Err: err}
	})
}
