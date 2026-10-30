package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func revealState(t *testing.T) (*State, []int, Config) {
	t.Helper()
	var yaml strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&yaml, "- name: cmd-%02d\n", i)
	}
	yaml.WriteString("- name: move-tab\n  children:\n    - name: session-a\n    - name: session-b\n")
	items, err := LoadYAMLFromString(yaml.String())
	if err != nil {
		t.Fatalf("LoadYAMLFromString: %v", err)
	}
	cfg := Config{Tiered: true}
	s, searchCols := NewState(items, cfg)
	ctx := s.TopCtx()
	ctx.TreeExpanded = make(map[int]bool)
	ctx.QueryExpanded = make(map[int]bool)
	ctx.TreeCursor = -1
	return s, searchCols, cfg
}

// A folder opened in place (the tree view, not the scoped one) is under the
// cursor and asks the renderer to place it, so the rows it opened are on
// screen (FZT-55).
func TestOpeningAFolderRevealsIt(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyRight} {
		t.Run(fmt.Sprint(key), func(t *testing.T) {
			s, searchCols, cfg := revealState(t)
			ctx := s.TopCtx()
			ctx.TreeCursor = 30
			ctx.NavMode = true

			HandleUnifiedKey(s, key, 0, false, cfg, searchCols)

			visible := TreeVisibleItems(s)
			if ctx.TreeCursor != 30 || !ctx.TreeReveal {
				t.Fatalf("cursor %d, reveal %v; want the folder's row, 30, revealed", ctx.TreeCursor, ctx.TreeReveal)
			}
			if got := visible[ctx.TreeCursor+1].Item.Fields[0]; got != "session-a" {
				t.Fatalf("row after the folder is %q, want session-a", got)
			}
		})
	}
}

// Typing moves the cursor to the top match and asks for it to be placed;
// an arrow key moves it without asking.
func TestSearchRevealsTheTopMatch(t *testing.T) {
	s, searchCols, cfg := revealState(t)
	ctx := s.TopCtx()
	for _, ch := range "move" {
		HandleUnifiedKey(s, tcell.KeyRune, ch, false, cfg, searchCols)
	}
	visible := TreeVisibleItems(s)
	if ctx.TreeCursor < 0 || visible[ctx.TreeCursor].Item.Fields[0] != "move-tab" || !ctx.TreeReveal {
		t.Fatalf("cursor %d, reveal %v; want move-tab revealed", ctx.TreeCursor, ctx.TreeReveal)
	}

	ctx.TreeReveal = false
	HandleUnifiedKey(s, tcell.KeyUp, 0, false, cfg, searchCols)
	if ctx.TreeReveal {
		t.Fatal("an arrow key asked for the cursor to be placed")
	}
}
