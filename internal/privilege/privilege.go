// Package privilege decides how (or whether) a write action can run.
//
// v1 policy, decided explicitly rather than deferred (see the plan): run as
// whatever privilege the invoking user already has. If already root, run
// directly. Otherwise, if sudo is available, hand the command to the UI
// suspend helper so sudo gets a real TTY for a password prompt if one is
// needed. If sudo isn't even installed, the action is not attempted and the
// UI shows a clear read-only banner instead of a swallowed failure.
//
// There is no credential storage, no daemon, and no setuid helper in v1.
package privilege

import (
	"context"
	"os"
	"time"

	"github.com/sabbakix/vegadesk/internal/execx"
)

// Status is resolved once at startup (EUID doesn't change at runtime, and
// re-probing passwordless sudo on every action would be wasteful).
type Status struct {
	IsRoot       bool
	HasSudo      bool // sudo binary present on PATH
	Passwordless bool // `sudo -n true` succeeded, i.e. no password needed
}

// Detect probes the current process's privilege posture.
func Detect(ctx context.Context) Status {
	st := Status{IsRoot: os.Geteuid() == 0}
	st.HasSudo = execx.LookPath("sudo")
	if !st.IsRoot && st.HasSudo {
		_, err := execx.RunTimeout(ctx, 2*time.Second, "sudo", "-n", "true")
		st.Passwordless = err == nil
	}
	return st
}

// CanAttemptWrite reports whether there is any path at all to run a
// privileged command (root, or sudo installed for an interactive prompt).
// false means the UI should show a read-only banner rather than an action.
func (s Status) CanAttemptWrite() bool {
	return s.IsRoot || s.HasSudo
}

// NeedsInteractiveAuth reports whether running argv will need a real TTY for
// a sudo password prompt (i.e. must go through the suspend helper rather
// than a plain background execx.Run).
func (s Status) NeedsInteractiveAuth() bool {
	return !s.IsRoot && !s.Passwordless
}

// NonInteractive returns argv prefixed for a background run: unchanged if
// root or if sudo needs a prompt (caller should route through suspend
// instead), or sudo-prefixed with -n when passwordless sudo is confirmed.
func (s Status) NonInteractive(argv []string) []string {
	if s.IsRoot || !s.HasSudo {
		return argv
	}
	return append([]string{"sudo", "-n"}, argv...)
}

// Interactive returns argv prefixed for a foreground run through the UI
// suspend helper, which attaches a real TTY so a sudo password prompt works.
func (s Status) Interactive(argv []string) []string {
	if s.IsRoot {
		return argv
	}
	return append([]string{"sudo"}, argv...)
}

// ReadOnlyReason returns a human banner when no elevation path exists at
// all, or "" when writes can at least be attempted.
func (s Status) ReadOnlyReason() string {
	if s.CanAttemptWrite() {
		return ""
	}
	return "read-only: not running as root and sudo is not installed"
}
