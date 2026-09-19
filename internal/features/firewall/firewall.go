// Package firewall shows ufw status/rules and lets you enable/disable it.
// Adding or removing individual rules needs a port/protocol input form,
// which is a follow-up -- v1 covers the read view and the on/off toggle,
// which is already the single highest-value action on this screen.
//
// ufw itself requires root even to read status (there is no unprivileged
// `ufw status`), which is why this feature -- unlike Containers or VMs --
// has to authenticate before it can show anything at all on a non-root
// session. See internal/collectors/firewall's package doc for the same
// point from the collector side.
package firewall

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"vegadesk/internal/collectors/firewall"
	"vegadesk/internal/privilege"
	"vegadesk/internal/ui"
	"vegadesk/internal/ui/theme"
)

// Model is the Firewall section.
type Model struct {
	styles theme.Styles
	priv   privilege.Status

	status       firewall.Status
	loaded       bool
	needsAuth    bool
	confirm      *ui.Confirm
	actionStatus string
	err          error
}

// ModalActive reports whether the enable/disable confirmation is on screen.
func (m *Model) ModalActive() bool { return m.confirm.Active() }

// New builds a Firewall section.
func New(styles theme.Styles, priv privilege.Status) *Model {
	return &Model{styles: styles, priv: priv}
}

func (m *Model) Title() string { return "Firewall (ufw)" }

func (m *Model) Init() tea.Cmd { return m.sampleCmd() }

type sampleMsg struct {
	status    firewall.Status
	needsAuth bool
	err       error
}

// readArgv builds the argv for a read-only `ufw status verbose`: run
// directly if already root, else attempt a non-interactive sudo first so a
// session with cached/passwordless credentials never has to stop and ask.
func (m *Model) readArgv() []string {
	base := []string{"ufw", "status", "verbose"}
	if m.priv.IsRoot {
		return base
	}
	return append([]string{"sudo", "-n"}, base...)
}

func (m *Model) sampleCmd() tea.Cmd {
	argv := m.readArgv()
	needsAuthOnFail := m.priv.NeedsInteractiveAuth()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		st, err := firewall.Collect(ctx, argv)
		if err != nil && needsAuthOnFail {
			return sampleMsg{needsAuth: true}
		}
		return sampleMsg{status: st, err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sampleMsg:
		m.loaded = true
		m.needsAuth = msg.needsAuth
		m.status = msg.status
		m.err = msg.err
		return m, nil

	case ui.SuspendDoneMsg:
		return m, m.sampleCmd() // re-check after `sudo -v` (Authenticate) or an action

	case ui.PrivilegedDoneMsg:
		if msg.Err != nil {
			m.actionStatus = fmt.Sprintf("%s failed: %v", msg.Label, msg.Err)
		} else {
			m.actionStatus = msg.Label + ": done"
		}
		return m, m.sampleCmd()
	}

	if m.confirm.Active() {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case ui.ConfirmResultMsg:
		if !msg.Confirmed {
			m.actionStatus = "cancelled"
			return m, nil
		}
		return m, m.toggle(msg.ID == "enable")

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, m.sampleCmd()
		case "a":
			if m.needsAuth {
				return m, ui.Authenticate()
			}
		case "e":
			m.confirm = ui.NewConfirm("enable", "Enable firewall",
				"blocks all incoming traffic not explicitly allowed. If no rule allows this SSH session's port, enabling ufw can lock you out.", "")
		case "d":
			m.confirm = ui.NewConfirm("disable", "Disable firewall",
				"drops all firewall rules; the host becomes fully exposed on every port until re-enabled.", "")
		}
	}
	return m, nil
}

func (m *Model) toggle(enable bool) tea.Cmd {
	verb := "disable"
	if enable {
		verb = "enable"
	}
	// --force: `ufw enable` normally asks an interactive "may disrupt
	// existing ssh connections, proceed?" y/n question. Run through the
	// non-interactive sudo path (see privilege.NonInteractive), that prompt
	// would read from /dev/null and fail instead of proceeding, so skip it
	// explicitly -- the confirm dialog above is this tool's equivalent
	// warning, shown before we ever get here.
	return ui.RunPrivileged(m.priv, "ufw "+verb, []string{"ufw", "--force", verb})
}

func (m *Model) View() string {
	if !m.loaded {
		return m.styles.Muted.Render("checking ufw status...")
	}
	if m.needsAuth {
		return m.styles.Banner.Render("sudo authentication required to read ufw status") +
			"\n\n" + m.styles.Muted.Render("a: authenticate (sudo -v)  r: retry")
	}
	if m.err != nil {
		return m.styles.Danger.Render(fmt.Sprintf("collection error: %v", m.err))
	}

	state := m.styles.OK.Render("active")
	if !m.status.Active {
		state = m.styles.Danger.Render("inactive")
	}
	out := fmt.Sprintf("Status: %s\nDefault: %s (incoming), %s (outgoing)\n\n",
		state, m.status.DefaultIncoming, m.status.DefaultOutgoing)

	if len(m.status.Rules) == 0 {
		out += m.styles.Muted.Render("(no rules)") + "\n"
	} else {
		out += fmt.Sprintf("%-28s%-14s%s\n", "TO", "ACTION", "FROM")
		for _, r := range m.status.Rules {
			out += fmt.Sprintf("%-28s%-14s%s\n", r.To, r.Action, r.From)
		}
	}

	out += "\n" + m.styles.Muted.Render("e: enable  d: disable  r: refresh")
	if m.actionStatus != "" {
		out += "\n" + m.styles.Muted.Render(m.actionStatus)
	}
	if m.confirm.Active() {
		out += "\n\n" + m.confirm.View(m.styles, 60)
	}
	return out
}
