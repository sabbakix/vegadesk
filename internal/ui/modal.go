package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"vegadesk/internal/ui/theme"
)

// ConfirmResultMsg is emitted once the user answers a Confirm dialog.
type ConfirmResultMsg struct {
	ID        string // caller-supplied, so a feature can tell dialogs apart
	Confirmed bool
}

// Confirm is a reusable destructive-action dialog. With RequireText empty it
// is a plain y/n prompt; with RequireText set the user must type that exact
// string (e.g. a device or unit name) before Enter confirms -- used for
// actions that can't be undone (mkfs, unmount, VM destroy).
type Confirm struct {
	ID          string
	Title       string
	Message     string
	RequireText string

	input  textinput.Model
	active bool
}

// NewConfirm builds a dialog. id round-trips through ConfirmResultMsg so a
// feature with multiple possible confirmations can tell them apart.
func NewConfirm(id, title, message, requireText string) *Confirm {
	c := &Confirm{ID: id, Title: title, Message: message, RequireText: requireText}
	if requireText != "" {
		ti := textinput.New()
		ti.Placeholder = requireText
		ti.Focus()
		c.input = ti
	}
	c.active = true
	return c
}

// Active reports whether the dialog is still on screen.
func (c *Confirm) Active() bool { return c != nil && c.active }

// Update handles key input. It returns a tea.Cmd carrying ConfirmResultMsg
// once the user confirms or cancels; the caller should stop routing input to
// the dialog once Active() is false.
func (c *Confirm) Update(msg tea.Msg) (*Confirm, tea.Cmd) {
	if !c.active {
		return c, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		if c.RequireText != "" {
			var cmd tea.Cmd
			c.input, cmd = c.input.Update(msg)
			return c, cmd
		}
		return c, nil
	}

	if c.RequireText == "" {
		switch keyMsg.String() {
		case "y", "Y", "enter":
			c.active = false
			return c, resultCmd(c.ID, true)
		case "n", "N", "esc", "q":
			c.active = false
			return c, resultCmd(c.ID, false)
		}
		return c, nil
	}

	switch keyMsg.String() {
	case "esc":
		c.active = false
		return c, resultCmd(c.ID, false)
	case "enter":
		if c.input.Value() == c.RequireText {
			c.active = false
			return c, resultCmd(c.ID, true)
		}
		return c, nil
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(keyMsg)
	return c, cmd
}

func resultCmd(id string, confirmed bool) tea.Cmd {
	return func() tea.Msg { return ConfirmResultMsg{ID: id, Confirmed: confirmed} }
}

// View renders the dialog box. width is the available content width the
// caller wants it centered/fit within.
func (c *Confirm) View(s theme.Styles, width int) string {
	box := s.Border.Copy().
		BorderForeground(s.Pal.Red).
		Padding(1, 2).
		Width(min(width-4, 60))

	body := s.Danger.Render(c.Title) + "\n\n" + c.Message + "\n"
	if c.RequireText != "" {
		body += "\n" + s.Muted.Render(fmt.Sprintf("type %q to confirm, esc to cancel", c.RequireText)) + "\n"
		body += c.input.View()
	} else {
		body += "\n" + s.Muted.Render("y to confirm, n/esc to cancel")
	}
	return box.Render(body)
}
