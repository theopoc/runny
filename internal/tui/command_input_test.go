package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestCommandInputEditsAtCursor(t *testing.T) {
	model := NewModel(Options{Command: "echo ok"})
	model.openCommandOverlay()

	model, _ = updateSpecialKey(model, tea.KeyLeft)
	model, _ = updateSpecialKey(model, tea.KeyLeft)
	model, _ = updateKey(model, "n")

	if model.Command != "echo nok" {
		t.Fatalf("command = %q, want %q", model.Command, "echo nok")
	}
	if view := model.renderCommandOverlay(80, 20); !strings.Contains(stripANSI(view), "echo nok") || model.commandCursor != 6 {
		t.Fatalf("cursor should overlay character at insertion point:\n%s", stripANSI(view))
	}
}

func TestCommandInputSelectsCopiesCutsAndReplacesText(t *testing.T) {
	model := NewModel(Options{Command: "echo old"})
	model.openCommandOverlay()

	for range 3 {
		model, _ = updateModifiedKey(model, tea.KeyLeft, tea.ModShift)
	}
	view := model.renderCommandOverlay(80, 20)
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
	model.openCommandOverlay()

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
	model.openCommandOverlay()

	runes := []rune(model.Command)
	cursor := len(runes)
	start, end := commandInputViewport(runes, cursor, 8)
	view := string(runes[start:end])
	if view != "-suffix" {
		t.Fatalf("long command viewport at end = %q", view)
	}
	if strings.Contains(view, "prefix") {
		t.Fatalf("long command viewport should hide distant prefix:\n%s", view)
	}

	for range 6 {
		model, _ = updateSpecialKey(model, tea.KeyLeft)
	}
	start, end = commandInputViewport(runes, model.commandCursor, 8)
	view = string(runes[start:end])
	if view != "789-suff" || model.commandCursor != len([]rune(model.Command))-6 {
		t.Fatalf("viewport should follow cursor moved inside command:\n%s", view)
	}
}

func TestCommandInputExpandsToKeepLongFocusedCommandVisible(t *testing.T) {
	command := strings.Repeat("x", 76)
	model := NewModel(Options{Command: command})
	model.openCommandOverlay()
	model, _ = updateWindowSize(model, 80, 24)
	model.moveCommandCursorToEnd()

	view := stripANSI(model.renderCommandOverlay(80, 20))
	lines := strings.Split(view, "\n")
	if len(lines) <= 3 {
		t.Fatalf("focused long command should expand beyond one content row:\n%s", view)
	}
	if strings.Count(view, "x") != len(command) {
		t.Fatalf("focused expanded input should keep complete command visible:\n%s", view)
	}
	for _, line := range lines {
		if width := ansi.StringWidth(line); width > 80 {
			t.Fatalf("responsive command line width = %d, want <= 80:\n%s", width, view)
		}
	}
}

func TestCommandOverlayPinsActionsToBottom(t *testing.T) {
	model := NewModel(Options{Command: "echo ok"})
	model.openCommandOverlay()

	for _, test := range []struct {
		name   string
		width  int
		height int
	}{
		{name: "standard", width: 80, height: 20},
		{name: "minimum", width: 60, height: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			lines := strings.Split(stripANSI(model.renderCommandOverlay(test.width, test.height)), "\n")
			if len(lines) < 3 || !strings.Contains(lines[len(lines)-2], "enter run · esc cancel · ↑/↓ history") {
				t.Fatalf("actions should occupy last interior row:\n%s", strings.Join(lines, "\n"))
			}
		})
	}
}

func TestCommandInputGrowthKeepsViewWithinTerminalHeight(t *testing.T) {
	model := NewModel(Options{Command: strings.Repeat("long-command-", 40)})
	model.openCommandOverlay()
	model, _ = updateWindowSize(model, 80, 24)
	model.moveCommandCursorToEnd()

	view := stripANSI(model.View().Content)
	if height := len(strings.Split(view, "\n")); height > 24 {
		t.Fatalf("responsive view height = %d, want <= 24:\n%s", height, view)
	}
}

func TestCommandInputRowStaysHiddenBehindOverlay(t *testing.T) {
	model := NewModel(Options{Command: strings.Repeat("command-", 20)})
	model, _ = updateWindowSize(model, 80, 24)
	model.Focus = FocusTargets

	if collapsed := model.renderSubHeader(80); collapsed != "" {
		t.Fatalf("unfocused command row should be hidden: %q", collapsed)
	}

	model.openCommandOverlay()
	if background := model.renderSubHeader(80); background != "" {
		t.Fatalf("command row behind overlay should be hidden: %q", background)
	}
	overlay := stripANSI(model.renderCommandOverlay(80, 20))
	if !strings.Contains(overlay, "Run command") {
		t.Fatalf("command overlay missing:\n%s", overlay)
	}
}

func TestCommandInputRewrapsWithTerminalWidth(t *testing.T) {
	model := NewModel(Options{Command: strings.Repeat("x", 200)})
	model.openCommandOverlay()
	model.moveCommandCursorToEnd()

	wide, _, _ := model.renderWrappedCommandInput(104, 20)
	narrow, _, _ := model.renderWrappedCommandInput(68, 20)
	wideAgain, _, _ := model.renderWrappedCommandInput(104, 20)

	if len(narrow) <= len(wide) {
		t.Fatalf("narrow command rows = %d, want greater than wide rows %d", len(narrow), len(wide))
	}
	if len(wideAgain) != len(wide) {
		t.Fatalf("wide command rows after resize = %d, want %d", len(wideAgain), len(wide))
	}
}

func TestCommandInputVerticalViewportFollowsCursor(t *testing.T) {
	model := NewModel(Options{Command: strings.Repeat("x", 800)})
	model.openCommandOverlay()
	model, _ = updateWindowSize(model, 80, 24)

	model.setCommandCursor(0, false)
	atStart := stripANSI(model.renderCommandOverlay(80, 20))
	if !strings.Contains(atStart, "Run command ↓") {
		t.Fatalf("viewport at start should show content below:\n%s", atStart)
	}

	model.setCommandCursor(400, false)
	inMiddle := stripANSI(model.renderCommandOverlay(80, 20))
	if !strings.Contains(inMiddle, "Run command ↕") {
		t.Fatalf("viewport in middle should show content above and below:\n%s", inMiddle)
	}

	model.moveCommandCursorToEnd()
	atEnd := stripANSI(model.renderCommandOverlay(80, 20))
	if !strings.Contains(atEnd, "Run command ↑") {
		t.Fatalf("viewport at end should show content above:\n%s", atEnd)
	}
	panelHeight, _, _ := model.panelDimensions(80, 24)
	if panelHeight < 10 {
		t.Fatalf("panel height = %d, want at least 10", panelHeight)
	}
}

func TestCommandInputWrapsWideUnicodeByTerminalCells(t *testing.T) {
	command := strings.Repeat("界", 38) + strings.Repeat("👩‍💻", 4)
	model := NewModel(Options{Command: command})
	model.openCommandOverlay()
	model, _ = updateWindowSize(model, 80, 24)
	model.moveCommandCursorToEnd()

	view := stripANSI(model.renderCommandOverlay(80, 20))
	if strings.Count(view, "界") != 38 || strings.Count(view, "👩‍💻") != 4 {
		t.Fatalf("wrapped Unicode command lost graphemes:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if width := ansi.StringWidth(line); width > 80 {
			t.Fatalf("wrapped Unicode line width = %d, want <= 80:\n%s", width, view)
		}
	}
}

func TestCommandOverlayBlocksPanelMouseHitRegion(t *testing.T) {
	model := NewModel(Options{Command: strings.Repeat("x", 200), Targets: nil})
	model.openCommandOverlay()
	model, _ = updateWindowSize(model, 80, 24)
	if _, hit := model.paneFocusAt(1, 5); hit {
		t.Fatal("command overlay should block background panel focus")
	}
}

func TestCommandCursorDoesNotInsertDisplayCell(t *testing.T) {
	model := NewModel(Options{Command: "for seq in 1..3 ; do"})
	model.openCommandOverlay()
	model.moveCommandCursorToEnd()

	model, _ = updateSpecialKey(model, tea.KeyLeft)
	if got := stripANSI(model.renderCommandInputValue(80)); got != model.Command {
		t.Fatalf("moving cursor must not insert a display cell: got %q, want %q", got, model.Command)
	}
}

func TestCommandOptionArrowMovesByWord(t *testing.T) {
	for _, test := range []struct {
		name      string
		leftCode  rune
		rightCode rune
	}{
		{name: "modified arrows", leftCode: tea.KeyLeft, rightCode: tea.KeyRight},
		{name: "terminal meta aliases", leftCode: 'b', rightCode: 'f'},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewModel(Options{Command: "for seq in 1..3 ; do"})
			model.openCommandOverlay()
			model.moveCommandCursorToEnd()

			updated, _ := model.Update(tea.KeyPressMsg{Code: test.leftCode, Mod: tea.ModAlt})
			model = updated.(Model)
			if model.commandCursor != 18 {
				t.Fatalf("option+left cursor = %d, want start of previous word at 18", model.commandCursor)
			}

			updated, _ = model.Update(tea.KeyPressMsg{Code: test.rightCode, Mod: tea.ModAlt})
			model = updated.(Model)
			if model.commandCursor != len([]rune(model.Command)) {
				t.Fatalf("option+right cursor = %d, want command end %d", model.commandCursor, len([]rune(model.Command)))
			}
		})
	}
}

func TestCommandSelectionCopiesWithControlOrCommandC(t *testing.T) {
	for _, test := range []struct {
		name string
		mod  tea.KeyMod
	}{
		{name: "control", mod: tea.ModCtrl},
		{name: "command", mod: tea.ModSuper},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewModel(Options{Command: "echo selected"})
			model.openCommandOverlay()
			model.moveCommandCursorToEnd()
			model.moveCommandCursor(-8, true)

			_, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: test.mod})
			if cmd == nil {
				t.Fatalf("%s+c should copy selected command text", test.name)
			}
		})
	}
}

func TestCommandSelectionKeepsFollowingTextColor(t *testing.T) {
	originalStyle := commandInputStyle
	commandInputStyle = commandInputStyle.Underline(true)
	t.Cleanup(func() { commandInputStyle = originalStyle })
	model := NewModel(Options{Command: "for seq in"})
	model.openCommandOverlay()
	model.setCommandCursor(4, false)
	model.moveCommandCursor(3, true)

	rendered := model.renderCommandInputValue(80)
	if !regexp.MustCompile("\\x1b\\[[0-9;]*4[0-9;]*mi").MatchString(rendered) {
		t.Fatalf("text after selection should restore command color: %q", rendered)
	}
}

func TestCommandSelectionFromEndIncludesFirstCharacter(t *testing.T) {
	originalStyle := commandSelectionStyle
	commandSelectionStyle = commandSelectionStyle.Underline(true)
	t.Cleanup(func() { commandSelectionStyle = originalStyle })

	model := NewModel(Options{Command: "sdmlkq"})
	model.openCommandOverlay()
	model.moveCommandCursorToEnd()
	for range len([]rune(model.Command)) {
		model.moveCommandCursor(-1, true)
	}

	if got := model.selectedCommandText(); got != model.Command {
		t.Fatalf("selection = %q, want complete command %q", got, model.Command)
	}
	rendered := model.renderCommandInputValue(80)
	wantFirst := commandSelectionStyle.Render("s")
	if !strings.HasPrefix(rendered, wantFirst) {
		t.Fatalf("first selected character should keep selection style: got %q, want prefix %q", rendered, wantFirst)
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
