package core

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// tagState is a small tmux-like palette: commands tagged by what they act on
// and what they do, under folders that carry no tags.
func tagState(t *testing.T, tagMode bool) (*State, []int, Config) {
	t.Helper()
	items, err := LoadYAMLFromString(`
- name: clients
  children:
    - name: new-session
    - name: kill-session
    - name: switch-client
    - name: find-session
- name: windows
  children:
    - name: move-window
    - name: kill-window
    - name: fg-window
- name: panes
  children:
    - name: move-pane
- name: ":"
  children:
    - name: tags
`)
	if err != nil {
		t.Fatalf("LoadYAMLFromString: %v", err)
	}
	tags := map[string][]string{
		"new-session":   {"session", "new"},
		"kill-session":  {"session", "kill"},
		"switch-client": {"session", "client", "switch"},
		"move-window":   {"window", "move"},
		"kill-window":   {"window", "kill"},
		"move-pane":     {"pane", "move"},
		"find-session":  {"session", "find"},
		"fg-window":     {"window", "fg"},
	}
	for i := range items {
		items[i].Tags = tags[items[i].Fields[0]]
	}
	cfg := Config{Tiered: true, TagMode: tagMode, DepthPenalty: 5}
	s, searchCols := NewState(items, cfg)
	ctx := s.TopCtx()
	ctx.TreeExpanded = make(map[int]bool)
	ctx.QueryExpanded = make(map[int]bool)
	ctx.TreeCursor = -1
	return s, searchCols, cfg
}

func typeQuery(t *testing.T, s *State, cfg Config, searchCols []int, q string) {
	t.Helper()
	for _, ch := range q {
		HandleSearchKeyResult(s, tcell.KeyRune, ch, cfg, searchCols)
	}
}

func filteredNames(s *State) []string {
	var names []string
	for _, item := range s.TopCtx().Filtered {
		names = append(names, item.Fields[0])
	}
	return names
}

func TestTagSearchIntersectsResolvedTags(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	typeQuery(t, s, cfg, cols, "move")
	assertVisible(t, s, "move-window", "move-pane")

	typeQuery(t, s, cfg, cols, " win")
	assertVisible(t, s, "move-window")
	got := s.TopCtx().TagTerms
	want := []TagTerm{{"move", "move"}, {"win", "window"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("TagTerms = %v, want %v", got, want)
	}
}

func TestTagSearchFindsByTagNotInName(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	typeQuery(t, s, cfg, cols, "session")
	// switch-client has no "session" in its name; its tag finds it.
	assertVisible(t, s, "new-session", "kill-session", "switch-client", "find-session")
}

func TestTagSearchPartialTermNarrowsWithinResults(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	// Across the whole menu "f" is closest to "fg", which no session row
	// has; among session rows it is "find", so the list narrows, not empties.
	typeQuery(t, s, cfg, cols, "session f")
	assertVisible(t, s, "find-session")
	if got := s.TopCtx().TagTerms[1].Tag; got != "find" {
		t.Fatalf("f resolved to %q, want find", got)
	}
	typeQuery(t, s, cfg, cols, "i")
	assertVisible(t, s, "find-session")
}

func TestTagSearchUnresolvedTermLeavesNothing(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	typeQuery(t, s, cfg, cols, "kill zzz")
	assertVisible(t, s)
	terms := s.TopCtx().TagTerms
	if len(terms) != 2 || terms[1].Tag != "" {
		t.Fatalf("TagTerms = %v, want zzz unresolved", terms)
	}
}

func TestTagSearchResultsAreFlat(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	typeQuery(t, s, cfg, cols, "kill")
	for _, row := range TreeVisibleItems(s) {
		if row.Item.Depth != 0 {
			t.Fatalf("row %s has depth %d, want 0", row.Item.Fields[0], row.Item.Depth)
		}
	}
}

func TestTagSearchExactTagBeatsLongerOne(t *testing.T) {
	if got := resolveTag("pane", []string{"panes-extra", "pane"}); got != "pane" {
		t.Fatalf("resolveTag = %q, want pane", got)
	}
}

func TestTagSearchArrowsKeepQuery(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	typeQuery(t, s, cfg, cols, "move")
	HandleSearchKeyResult(s, tcell.KeyDown, 0, cfg, cols)
	if q := string(s.TopCtx().Query); q != "move" {
		t.Fatalf("query after Down = %q, want move", q)
	}
	assertVisible(t, s, "move-window", "move-pane")
}

func TestTagSearchTabCompletesTerm(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	typeQuery(t, s, cfg, cols, "ses")
	if g := TagGhost(s.TopCtx()); g != "sion" {
		t.Fatalf("ghost = %q, want sion", g)
	}
	HandleSearchKeyResult(s, tcell.KeyTab, 0, cfg, cols)
	if q := string(s.TopCtx().Query); q != "session " {
		t.Fatalf("query after Tab = %q, want %q", q, "session ")
	}
	assertVisible(t, s, "new-session", "kill-session", "switch-client", "find-session")
}

func TestTagSearchPaletteStaysReachable(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	typeQuery(t, s, cfg, cols, ":")
	if TagSearchActive(s.TopCtx()) {
		t.Fatal("a query starting with ':' ran a tag search")
	}
	if names := filteredNames(s); len(names) == 0 || names[0] != ":" {
		t.Fatalf("filtered = %v, want the : folder first", names)
	}
}

func TestTagModeOffSearchesAsBefore(t *testing.T) {
	s, cols, cfg := tagState(t, false)
	typeQuery(t, s, cfg, cols, "session")
	if TagSearchActive(s.TopCtx()) {
		t.Fatal("tag search ran with tag mode off")
	}
	for _, name := range filteredNames(s) {
		if name == "switch-client" {
			t.Fatal("switch-client matched by tag with tag mode off")
		}
	}
}

func TestTagModeUntaggedMenuSearchesAsBefore(t *testing.T) {
	items, err := LoadYAMLFromString(`
- name: merge requests
- name: pipelines
`)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Tiered: true, TagMode: true}
	s, cols := NewState(items, cfg)
	s.TopCtx().TreeExpanded = make(map[int]bool)
	typeQuery(t, s, cfg, cols, "merge")
	if TagSearchActive(s.TopCtx()) {
		t.Fatal("tag search ran on a menu with no tags")
	}
	if names := filteredNames(s); len(names) != 1 || !strings.HasPrefix(names[0], "merge") {
		t.Fatalf("filtered = %v, want merge requests", names)
	}
}
