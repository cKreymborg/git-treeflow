package tui

import "github.com/charmbracelet/bubbles/key"

type listKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Select   key.Binding
	Create   key.Binding
	Delete   key.Binding
	Prune    key.Binding
	Settings key.Binding
	Help     key.Binding
	Quit     key.Binding
}

var listKeys = listKeyMap{
	Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/up", "up")),
	Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/down", "down")),
	Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
	Select:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "switch to worktree")),
	Create:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "create worktree")),
	Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete worktree")),
	Prune:    key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prune stale")),
	Settings: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "settings")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:     key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "quit")),
}
