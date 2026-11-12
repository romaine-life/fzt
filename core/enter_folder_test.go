package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestEnterFolderOpensInsideAndBacksOutToRoot(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		s, searchCols, cfg := scopedState(t, false)
		s.ScopedView = scoped
		cfg.ScopedView = scoped

		if !EnterFolder(s, []string{"repos", "fzt"}, cfg, searchCols) {
			t.Fatalf("scoped=%v: EnterFolder reported repos/fzt missing", scoped)
		}
		ctx := s.TopCtx()
		if len(ctx.Scope) != 3 {
			t.Fatalf("scoped=%v: scope depth = %d, want 3", scoped, len(ctx.Scope))
		}
		typeText(t, s, cfg, searchCols, "games")
		for _, item := range s.TopCtx().Filtered {
			if item.Fields[0] == "games" {
				t.Fatalf("scoped=%v: search inside repos/fzt reached the root's games", scoped)
			}
		}

		// Escape unwinds one layer per press (the query, then each folder),
		// so the root comes before the menu would close.
		for i := 0; i < 6 && len(s.TopCtx().Scope) > 1; i++ {
			press(t, s, cfg, searchCols, tcell.KeyEscape)
		}
		if got := len(s.TopCtx().Scope); got != 1 || s.Cancelled {
			t.Fatalf("scoped=%v: after backing out, scope depth = %d, cancelled = %v; want the root, still open", scoped, got, s.Cancelled)
		}
		press(t, s, cfg, searchCols, tcell.KeyEscape)
		if !s.Cancelled {
			t.Fatalf("scoped=%v: Escape at the root didn't close the menu", scoped)
		}
	}
}

func TestEnterFolderMissingLeavesTheRoot(t *testing.T) {
	for _, path := range [][]string{{"tmux"}, {"repos", "nope"}, {"repos", "fzt", "core", "deep"}} {
		s, searchCols, cfg := scopedState(t, false)
		if EnterFolder(s, path, cfg, searchCols) {
			t.Fatalf("EnterFolder(%v) reported found", path)
		}
		if got := len(s.TopCtx().Scope); got != 1 {
			t.Fatalf("EnterFolder(%v) left scope depth %d, want 1", path, got)
		}
	}
}

// StartRow starts the cursor on that entry of the folder opened, and the
// arrow keys go on from there; past the last entry it starts on the last.
func TestEnterFolderStartRow(t *testing.T) {
	items, err := LoadYAMLFromString(`
- name: other
- name: tmux
  children:
    - name: nav
      children:
        - name: window
          children:
            - name: w1
            - name: w2
            - name: w3
            - name: w4
- name: last
`)
	if err != nil {
		t.Fatalf("LoadYAMLFromString: %v", err)
	}
	for _, scoped := range []bool{false, true} {
		for _, tc := range []struct {
			row        int
			want, next string
		}{{3, "w3", "w4"}, {1, "w1", "w2"}, {9, "w4", ""}} {
			cfg := Config{Tiered: true, ScopedView: scoped, StartRow: tc.row}
			s, searchCols := NewState(items, cfg)
			ctx := s.TopCtx()
			ctx.TreeExpanded = make(map[int]bool)
			ctx.QueryExpanded = make(map[int]bool)
			ctx.TreeCursor = -1
			if !EnterFolder(s, []string{"tmux", "nav", "window"}, cfg, searchCols) {
				t.Fatalf("scoped=%v: EnterFolder reported tmux/nav/window missing", scoped)
			}
			cursorName := func() string {
				visible := TreeVisibleItems(s)
				if ctx.TreeCursor < 0 || ctx.TreeCursor >= len(visible) {
					return ""
				}
				return visible[ctx.TreeCursor].Item.Fields[0]
			}
			if got := cursorName(); got != tc.want {
				t.Fatalf("scoped=%v, StartRow %d: cursor on %q, want %q", scoped, tc.row, got, tc.want)
			}
			if tc.next == "" {
				continue
			}
			press(t, s, cfg, searchCols, tcell.KeyDown)
			if got := cursorName(); got != tc.next {
				t.Fatalf("scoped=%v, StartRow %d: Down moved to %q, want %q", scoped, tc.row, got, tc.next)
			}
		}
	}
}
