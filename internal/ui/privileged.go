package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sabbakix/vegadesk/internal/execx"
	"github.com/sabbakix/vegadesk/internal/privilege"
)

// PrivilegedDoneMsg is sent once a RunPrivileged command finishes.
type PrivilegedDoneMsg struct {
	Label string
	Err   error
}

// RunPrivileged is the one place every write action across every feature
// (Storage, Services, Containers, VMs, Users, Firewall) goes to run a
// command that might need root: it runs directly as root, silently via
// `sudo -n` when passwordless sudo was confirmed at startup, or through
// Suspend (a real TTY) when sudo will need to prompt for a password.
//
// Callers should already have gated the action on priv.CanAttemptWrite()
// (the sidebar shows a read-only banner otherwise); RunPrivileged itself
// just picks *how* to run, not *whether* to.
func RunPrivileged(priv privilege.Status, label string, argv []string) tea.Cmd {
	if priv.NeedsInteractiveAuth() {
		return Suspend(label, priv.Interactive(argv))
	}
	full := priv.NonInteractive(argv)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := execx.Run(ctx, full[0], full[1:]...)
		return PrivilegedDoneMsg{Label: label, Err: err}
	}
}

// Authenticate prompts for and caches sudo credentials via a real TTY
// (`sudo -v`), so a subsequent non-interactive `sudo -n` read can succeed.
// This is for read-only collectors that themselves require root (e.g. `ufw
// status`), which RunPrivileged doesn't cover since it only runs actions
// that don't need to capture parseable output back.
func Authenticate() tea.Cmd {
	return Suspend("authenticate", []string{"sudo", "-v"})
}
