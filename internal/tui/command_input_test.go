package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCommandInputEditsAtCursor(t *testing.T) {
	model := NewModel(Options{Command: "echo ok"})
	model.Focus = FocusCommand

	model, _ = updateSpecialKey(model, tea.KeyLeft)
	model, _ = updateSpecialKey(model, tea.KeyLeft)
	model, _ = updateKey(model, "n")

	if model.Command != "echo nok" {
		t.Fatalf("command = %q, want %q", model.Command, "echo nok")
	}
	if plain := stripANSI(model.renderSubHeader(80)); !strings.Contains(plain, "echo n▌ok") {
		t.Fatalf("cursor should render at insertion point:\n%s", plain)
	}
}

func TestCommandInputSelectsCopiesCutsAndReplacesText(t *testing.T) {
	model := NewModel(Options{Command: "echo old"})
	model.Focus = FocusCommand

	for range 3 {
		model, _ = updateModifiedKey(model, tea.KeyLeft, tea.ModShift)
	}
	view := model.renderSubHeader(80)
	got := model.selectedCommandText()
	if !strings.Contains(view, "\x1b[") || got != "old" {
		t.Fatalf("selected text = %q, want old; view:\n%s", got, view)
	}

	var copyCmd tea.Cmd
	model, copyCmd = updateModifiedKey(model, 'c', tea.ModCtrl)
	if copyCmd == nil {
		t.Fatal("ctrl+c with selection should write clipboard")
	}
	if _, ok := copyCmd().(tea.QuitMsg); ok {
		t.Fatal("ctrl+c with selection should not quit")
	}

	model, _ = updateModifiedKey(model, 'x', tea.ModCtrl)
	if model.Command != "echo " {
		t.Fatalf("command after cut = %q, want %q", model.Command, "echo ")
	}
	model, _ = updatePaste(model, "new")
	if model.Command != "echo new" {
		t.Fatalf("command after paste = %q, want %q", model.Command, "echo new")
	}
}

func TestCommandInputReadsClipboardAndHandlesUnicode(t *testing.T) {
	model := NewModel(Options{Command: "echo café"})
	model.Focus = FocusCommand

	model, _ = updateSpecialKey(model, tea.KeyLeft)
	model, cmd := updateModifiedKey(model, 'v', tea.ModCtrl)
	if cmd == nil {
		t.Fatal("ctrl+v should request clipboard")
	}
	updated, _ := model.Update(tea.ClipboardMsg{Content: " très"})
	model = updated.(Model)
	if model.Command != "echo caf trèsé" {
		t.Fatalf("command = %q, want unicode insertion at cursor", model.Command)
	}
}

func TestCommandInputKeepsCursorVisibleWhenCommandExceedsWidth(t *testing.T) {
	model := NewModel(Options{Command: "prefix-0123456789-suffix"})
	model.Focus = FocusCommand

	view := stripANSI(model.renderSubHeader(16))
	if !strings.Contains(view, "suffix▌") {
		t.Fatalf("long command should scroll to cursor at end:\n%s", view)
	}
	if strings.Contains(view, "prefix") {
		t.Fatalf("long command viewport should hide distant prefix:\n%s", view)
	}

	for range 6 {
		model, _ = updateSpecialKey(model, tea.KeyLeft)
	}
	view = stripANSI(model.renderSubHeader(16))
	if !strings.Contains(view, "▌suffi") {
		t.Fatalf("viewport should follow cursor moved inside command:\n%s", view)
	}
}

func updateModifiedKey(model Model, code rune, mod tea.KeyMod) (Model, tea.Cmd) {
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: code, Mod: mod}))
	return updated.(Model), cmd
}

func updatePaste(model Model, content string) (Model, tea.Cmd) {
	updated, cmd := model.Update(tea.PasteMsg{Content: content})
	return updated.(Model), cmd
}
