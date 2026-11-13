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
