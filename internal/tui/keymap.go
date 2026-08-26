package tui

import "charm.land/bubbles/v2/key"

type keyName string

func (k keyName) String() string {
	return string(k)
}

type keyMap struct {
	Escape        key.Binding
	Quit          key.Binding
	ConfirmQuit   key.Binding
	Help          key.Binding
	Command       key.Binding
	Palette       key.Binding
	Options       key.Binding
	Run           key.Binding
	Up            key.Binding
	Down          key.Binding
	NextMatch     key.Binding
	PreviousMatch key.Binding
	First         key.Binding
	Last          key.Binding
	NextPane      key.Binding
	PreviousPane  key.Binding
	Filter        key.Binding
	History       key.Binding
	Zoom          key.Binding
	ToggleTarget  key.Binding
	ToggleVisible key.Binding
	Unfold        key.Binding
	Fold          key.Binding
	Cancel        key.Binding
	RerunFailed   key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	HalfPageUp    key.Binding
	HalfPageDown  key.Binding
	Follow        key.Binding
}

var defaultKeys = newKeyMap()

func newKeyMap() keyMap {
	return keyMap{
		Escape:        newBinding([]string{"esc"}, "esc", "close mode"),
		Quit:          newBinding([]string{"q"}, "q", "quit"),
		ConfirmQuit:   newBinding([]string{"ctrl+c"}, "ctrl+c", "confirm quit"),
		Help:          newBinding([]string{"?"}, "?", "keymap"),
		Command:       newBinding([]string{":"}, ":", "run command"),
		Palette:       newBinding([]string{"ctrl+p"}, "ctrl+p", "palette"),
		Options:       newBinding([]string{"o"}, "o", "options"),
		Run:           newBinding([]string{"enter"}, "enter", "run selected"),
		Up:            newBinding([]string{"up", "k"}, "up/k", "move up"),
		Down:          newBinding([]string{"down", "j"}, "down/j", "move down"),
		NextMatch:     newBinding([]string{"n"}, "n", "next match"),
		PreviousMatch: newBinding([]string{"N"}, "N", "previous match"),
		First:         newBinding([]string{"home", "g"}, "home/g", "first task"),
		Last:          newBinding([]string{"end", "G"}, "end/G", "last task"),
		NextPane:      newBinding([]string{"tab"}, "tab", "tasks/output"),
		PreviousPane:  newBinding([]string{"shift+tab"}, "shift+tab", "previous pane"),
		Filter:        newBinding([]string{"/"}, "/", "filter"),
		History:       newBinding([]string{"H"}, "H", "history"),
		Zoom:          newBinding([]string{"z"}, "z", "maximize panel / split"),
		ToggleTarget:  newBinding([]string{" ", "space"}, "space", "toggle select tree"),
		ToggleVisible: newBinding([]string{"a"}, "a", "toggle visible/matches"),
		Unfold:        newBinding([]string{"right", "l"}, "right/l", "unfold"),
		Fold:          newBinding([]string{"left", "h"}, "left/h", "fold"),
		Cancel:        newBinding([]string{"delete", "x"}, "del/x", "cancel selected"),
		RerunFailed:   newBinding([]string{"R"}, "R", "rerun failed"),
		PageUp:        newBinding([]string{"pageup", "ctrl+b"}, "pageup/ctrl+b", "scroll up"),
		PageDown:      newBinding([]string{"pagedown", "ctrl+f"}, "pagedown/ctrl+f", "scroll down"),
		HalfPageUp:    newBinding([]string{"ctrl+u"}, "ctrl+u", "half up"),
		HalfPageDown:  newBinding([]string{"ctrl+d"}, "ctrl+d", "half down"),
		Follow:        newBinding([]string{"f"}, "f", "tail"),
	}
}

func newBinding(keys []string, helpKey, description string) key.Binding {
	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(helpKey, description),
	)
}

func matchesKey(name string, bindings ...key.Binding) bool {
	return key.Matches(keyName(name), bindings...)
}

func helpBindingFrom(binding key.Binding) helpBinding {
	help := binding.Help()
	return helpBinding{key: help.Key, description: help.Desc}
}

func helpBindingFromKey(binding key.Binding, index int, description string) helpBinding {
	keys := binding.Keys()
	if index < 0 || index >= len(keys) {
		return helpBindingFrom(binding)
	}
	return helpBinding{key: keys[index], description: description}
}
