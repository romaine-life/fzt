package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func scopedState(t *testing.T, foldersOnly bool) (*State, []int, Config) {
	t.Helper()
	items, err := LoadYAMLFromString(`
- name: repos
  children:
    - name: fzt
      children:
        - name: core
          children:
            - name: deep
    - name: fzt-picker
      children:
        - name: windows
- name: games
  children:
    - name: spire
`)
	if err != nil {
		t.Fatalf("LoadYAMLFromString: %v", err)
	}
	cfg := Config{Tiered: true, ScopedView: true, FoldersOnly: foldersOnly}
	s, searchCols := NewState(items, cfg)
	ctx := s.TopCtx()
	ctx.TreeExpanded = make(map[int]bool)
	ctx.QueryExpanded = make(map[int]bool)
	ctx.TreeCursor = -1
	return s, searchCols, cfg
}

func visibleNames(s *State) []string {
	var names []string
	for _, row := range TreeVisibleItems(s) {
		names = append(names, row.Item.Fields[0])
	}
	return names
}

func assertVisible(t *testing.T, s *State, want ...string) {
	t.Helper()
	got := visibleNames(s)
	if len(got) != len(want) {
		t.Fatalf("visible = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("visible = %v, want %v", got, want)
		}
	}
}

func press(t *testing.T, s *State, cfg Config, searchCols []int, key tcell.Key) ActionResult {
	t.Helper()
	return HandleUnifiedKeyResult(s, key, 0, false, cfg, searchCols)
}

func typeText(t *testing.T, s *State, cfg Config, searchCols []int, text string) {
	t.Helper()
	for _, ch := range text {
		if r := HandleUnifiedKeyResult(s, tcell.KeyRune, ch, false, cfg, searchCols); r.Action != "" {
			t.Fatalf("typing %q emitted %q", ch, r.Action)
		}
	}
}

func TestScopedViewShowsOnlyCurrentFolder(t *testing.T) {
	s, searchCols, cfg := scopedState(t, false)
	assertVisible(t, s, "repos", "games")

	press(t, s, cfg, searchCols, tcell.KeyDown)
	press(t, s, cfg, searchCols, tcell.KeyEnter)
	assertVisible(t, s, "fzt", "fzt-picker")
	if s.TopCtx().TreeCursor != -1 {
		t.Fatalf("cursor = %d after entering, want -1", s.TopCtx().TreeCursor)
	}

	press(t, s, cfg, searchCols, tcell.KeyDown)
	press(t, s, cfg, searchCols, tcell.KeyRight)
	assertVisible(t, s, "core")

	press(t, s, cfg, searchCols, tcell.KeyLeft)
	assertVisible(t, s, "fzt", "fzt-picker")
	press(t, s, cfg, searchCols, tcell.KeyBackspace2)
	assertVisible(t, s, "repos", "games")
}

func TestScopedViewSearchCoversDirectChildrenOnly(t *testing.T) {
	s, searchCols, cfg := scopedState(t, false)
	press(t, s, cfg, searchCols, tcell.KeyDown)
	press(t, s, cfg, searchCols, tcell.KeyEnter)

	typeText(t, s, cfg, searchCols, "core")
	for _, item := range s.TopCtx().Filtered {
		if item.Fields[0] == "core" {
			t.Fatal("search reached a grandchild of the current folder")
		}
	}
}

func TestScopedViewFileModeEnterOnTypedFolderEntersIt(t *testing.T) {
	s, searchCols, cfg := scopedState(t, false)
	typeText(t, s, cfg, searchCols, "repos")
	press(t, s, cfg, searchCols, tcell.KeyEnter)
	typeText(t, s, cfg, searchCols, "picker")
	if r := press(t, s, cfg, searchCols, tcell.KeyEnter); r.Action != "" {
		t.Fatalf("Enter on matched folder = %q, want it entered", r.Action)
	}
	assertVisible(t, s, "windows")
}

func TestScopedViewFoldersOnlyEnterPicksHighlightedFolder(t *testing.T) {
	s, searchCols, cfg := scopedState(t, true)
	press(t, s, cfg, searchCols, tcell.KeyDown)
	r := press(t, s, cfg, searchCols, tcell.KeyEnter)
	if r.Action != "select:repos" || !r.HasItem || r.Item.Fields[0] != "repos" {
		t.Fatalf("Enter on highlighted folder = %q (item %v), want select:repos", r.Action, r.Item.Fields)
	}
}

func TestScopedViewFoldersOnlyEnterPicksTypedFolder(t *testing.T) {
	s, searchCols, cfg := scopedState(t, true)
	typeText(t, s, cfg, searchCols, "repos")
	press(t, s, cfg, searchCols, tcell.KeyRight)
	assertVisible(t, s, "fzt", "fzt-picker")
	typeText(t, s, cfg, searchCols, "picker")
	r := press(t, s, cfg, searchCols, tcell.KeyEnter)
	if r.Action != "select:fzt-picker" {
		t.Fatalf("Enter on typed folder = %q, want select:fzt-picker", r.Action)
	}
}

func TestScopedViewFoldersOnlyEnterPicksCurrentFolder(t *testing.T) {
	s, searchCols, cfg := scopedState(t, true)
	if r := press(t, s, cfg, searchCols, tcell.KeyEnter); r.Action != "" {
		t.Fatalf("Enter at root with no cursor = %q, want no action", r.Action)
	}

	press(t, s, cfg, searchCols, tcell.KeyDown)
	press(t, s, cfg, searchCols, tcell.KeyRight)
	assertVisible(t, s, "fzt", "fzt-picker")
	r := press(t, s, cfg, searchCols, tcell.KeyEnter)
	if r.Action != "select:repos" || !r.HasItem || r.Item.Fields[0] != "repos" {
		t.Fatalf("Enter in a folder with nothing highlighted = %q (item %v), want select:repos", r.Action, r.Item.Fields)
	}
}

func TestScopedViewWithoutFoldersOnlyEnterOnNothingIsNoop(t *testing.T) {
	s, searchCols, cfg := scopedState(t, false)
	press(t, s, cfg, searchCols, tcell.KeyDown)
	press(t, s, cfg, searchCols, tcell.KeyEnter)
	if r := press(t, s, cfg, searchCols, tcell.KeyEnter); r.Action != "" {
		t.Fatalf("Enter with no cursor = %q, want no action", r.Action)
	}
	assertVisible(t, s, "fzt", "fzt-picker")
}

func TestScopedViewClickEntersFolder(t *testing.T) {
	s, _, cfg := scopedState(t, false)
	// No border, no headers: items start below the 3-row prompt bar.
	if r := ClickUnifiedRowResult(s, 3, cfg, 24); r.Action != "" {
		t.Fatalf("click on folder = %q, want it entered", r.Action)
	}
	assertVisible(t, s, "fzt", "fzt-picker")
}

func TestQueryWithNoMatchLeavesNothingUnderCursor(t *testing.T) {
	s, searchCols, cfg := scopedState(t, true)
	press(t, s, cfg, searchCols, tcell.KeyDown)
	press(t, s, cfg, searchCols, tcell.KeyRight)

	typeText(t, s, cfg, searchCols, "fzt-p")
	typeText(t, s, cfg, searchCols, "qqq")
	if c := s.TopCtx().TreeCursor; c != -1 {
		t.Fatalf("cursor = %d with no matches, want -1", c)
	}
	if r := press(t, s, cfg, searchCols, tcell.KeyEnter); r.Action != "" {
		t.Fatalf("Enter with no matches = %q, want no action", r.Action)
	}
	assertVisible(t, s, "fzt", "fzt-picker")
}
