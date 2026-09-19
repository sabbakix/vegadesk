package app

import "github.com/charmbracelet/bubbles/key"

// globalKeys are handled by the root model before a key is ever offered to
// the active section, so every section gets consistent navigation for free.
type globalKeyMap struct {
	Quit key.Binding
	// ForceQuit is ctrl+c alone, checked even while a section has a modal
	// open (unlike Quit's "q", which a modal treats as cancel instead). A
	// terminal program that eats ctrl+c reads as hung.
	ForceQuit key.Binding
	Help      key.Binding
	NextTab   key.Binding
	PrevTab   key.Binding
	JumpKeys  []key.Binding // "1".."9","0" -> section index, built in newGlobalKeys
}

func newGlobalKeys() globalKeyMap {
	digits := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}
	jumps := make([]key.Binding, len(digits))
	for i, d := range digits {
		jumps[i] = key.NewBinding(key.WithKeys(d))
	}
	return globalKeyMap{
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		// Deliberately not "h"/"l": several sections bind those to their own
		// actions (follow logs, lock user), and vim-style single-letter
		// nav would silently steal them with no indication why.
		NextTab:  key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab", "next section")),
		PrevTab:  key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("shift+tab", "prev section")),
		JumpKeys: jumps,
	}
}

// ShortHelp implements help.KeyMap for the footer help bar.
func (k globalKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.NextTab, k.PrevTab, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k globalKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
