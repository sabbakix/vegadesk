// Command vegadesk is a single-host, btop-styled TUI for managing a Linux
// server over SSH: system dashboard, storage, services, containers, VMs,
// networking, logs, users, and firewall -- no web server required.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/sabbakix/vegadesk/internal/app"
	"github.com/sabbakix/vegadesk/internal/execx"
	"github.com/sabbakix/vegadesk/internal/platform"
	"github.com/sabbakix/vegadesk/internal/privilege"
	"github.com/sabbakix/vegadesk/internal/ui/theme"
)

func main() {
	colorFlag := flag.String("color", "auto", "color profile: auto, 256, or truecolor (override for terminals that under-report support, common over SSH/tmux)")
	flag.Parse()

	switch *colorFlag {
	case "auto":
		// leave lipgloss's TERM/COLORTERM auto-detection in place
	case "256":
		lipgloss.SetColorProfile(termenv.ANSI256)
	case "truecolor":
		lipgloss.SetColorProfile(termenv.TrueColor)
	default:
		fmt.Fprintf(os.Stderr, "vegadesk: unknown --color value %q (want auto, 256, or truecolor)\n", *colorFlag)
		os.Exit(2)
	}

	ctx := context.Background()
	caps := platform.Detect(execx.LookPath)
	priv := privilege.Detect(ctx)
	styles := theme.New(theme.Default)

	root := app.New(styles, caps, priv)
	registerSections(root, caps, priv, styles)

	p := tea.NewProgram(root, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "vegadesk: %v\n", err)
		os.Exit(1)
	}
}
