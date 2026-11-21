package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

var ctrlU = keyPress{key: tcell.KeyCtrlU}

// Ctrl+U empties the search box and leaves you typing where you are: at the
// top, inside a folder, or from normal mode.
func TestCtrlUClearsTheSearch(t *testing.T) {
	for _, tc := range []struct {
		name     string
		f        keyFixture
		inFolder bool
	}{
		{name: "search", f: keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: runes("wso")}},
		{name: "normal mode", f: keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: seq(runes("wso"), keys(tcell.KeyEscape))}},
		{name: "inside folder", f: keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: seq(runes("workspace"), runes(" "), runes("wsl"))}, inFolder: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, cols := newKeysState(t, tc.f)
			pressKey(s, tc.f, ctrlU, cols)
			ctx := s.TopCtx()
			if len(ctx.Query) != 0 || ctx.NavMode {
				t.Fatalf("after Ctrl+U: query %q, normal mode %v; want empty and typing", string(ctx.Query), ctx.NavMode)
			}
			if got := len(ctx.Scope) > 1; got != tc.inFolder {
				t.Fatalf("after Ctrl+U: in a folder = %v, want %v", got, tc.inFolder)
			}
			pressKey(s, tc.f, keyPress{key: tcell.KeyRune, ch: 'l'}, cols)
			if string(ctx.Query) != "l" {
				t.Fatalf("typing after Ctrl+U: query %q, want l", string(ctx.Query))
			}
		})
	}
}

// In the flat list, whose search box has a text cursor, Ctrl+U deletes what
// is before the cursor and keeps what is after, as a shell's does.
func TestCtrlUFlatKeepsTextAfterCursor(t *testing.T) {
	f := keyFixture{cfg: Config{Nth: []int{1}}, flat: true, steps: seq(runes("wsorg"), keys(tcell.KeyLeft, tcell.KeyLeft, tcell.KeyLeft))} // the first Left leaves the list for the box
	s, cols := newKeysState(t, f)
	pressKey(s, f, ctrlU, cols)
	if ctx := s.TopCtx(); string(ctx.Query) != "rg" || ctx.Cursor != 0 {
		t.Fatalf("after Ctrl+U: query %q cursor %d, want rg and 0", string(ctx.Query), ctx.Cursor)
	}
}

// A text box an app took over (Live.Ask, an input entry) erases the same way,
// a prefilled answer still selected included.
func TestCtrlUInPromptBox(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg()}
	s, cols := newKeysState(t, f)
	s.PromptMode, s.PromptAction = "ask", "ask"
	s.PromptQuery, s.PromptCursor, s.PromptSelected = []rune("old answer"), 0, 10
	pressKey(s, f, ctrlU, cols)
	if string(s.PromptQuery) != "" || s.PromptCursor != 0 || s.PromptSelected != 0 {
		t.Fatalf("selected answer after Ctrl+U: %q cursor %d selected %d", string(s.PromptQuery), s.PromptCursor, s.PromptSelected)
	}
	s.PromptQuery, s.PromptCursor = []rune("new text"), 3
	pressKey(s, f, ctrlU, cols)
	if string(s.PromptQuery) != " text" || s.PromptCursor != 0 {
		t.Fatalf("after Ctrl+U mid-text: %q cursor %d, want \" text\" and 0", string(s.PromptQuery), s.PromptCursor)
	}
}

// In a rename or property edit, Ctrl+U empties what has been typed.
func TestCtrlUInRename(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg()}
	s, cols := newKeysState(t, f)
	s.EditMode, s.EditBuffer = "rename", []rune("lights")
	pressKey(s, f, ctrlU, cols)
	if len(s.EditBuffer) != 0 || s.EditMode != "rename" {
		t.Fatalf("after Ctrl+U: buffer %q, mode %q; want empty, still renaming", string(s.EditBuffer), s.EditMode)
	}
}
