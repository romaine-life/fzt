package core

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"gopkg.in/yaml.v3"
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

func TestTagSearchFindsARowByItsName(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	// "move-window" is no tag, but it is the row's name.
	typeQuery(t, s, cfg, cols, "move-window")
	assertVisible(t, s, "move-window")
	// An untagged row (a folder) is found by its name too.
	s, cols, cfg = tagState(t, true)
	typeQuery(t, s, cfg, cols, "panes")
	assertVisible(t, s, "panes")
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

func TestTagsLoadSaveAndEdit(t *testing.T) {
	items, err := LoadYAMLFromString(`
- name: git
  children:
    - name: gitprune
      action: gitprune
      tags: [git, branch, cleanup]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatTags(items[1].Tags); got != "git branch cleanup" {
		t.Fatalf("loaded tags = %q", got)
	}
	s, _ := NewState(items, Config{Tiered: true})
	ctx := s.TopCtx()

	// The item editor's tags property: spaces or commas, repeats dropped.
	ctx.AllItems = append(ctx.AllItems, Item{Fields: []string{"tags", "git branch cleanup"}, IsProperty: true, PropertyOf: 1, PropertyKey: "tags"})
	prop := len(ctx.AllItems) - 1
	s.EditMode, s.EditTargetIdx, s.EditBuffer = "rename", prop, []rune("git, prune  git")
	handleRenameKey(s, tcell.KeyEnter, 0)
	if got := FormatTags(ctx.AllItems[1].Tags); got != "git prune" || ctx.AllItems[prop].Fields[1] != "git prune" {
		t.Fatalf("edited tags = %q, shown %q", got, ctx.AllItems[prop].Fields[1])
	}

	// Saving keeps them, and the saved tree loads back with them.
	saved := SerializeTree(ctx)
	out, err := yaml.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	again, err := LoadYAMLFromString(string(out))
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatTags(again[1].Tags); got != "git prune" {
		t.Fatalf("tags after save and reload = %q\n%s", got, out)
	}
}

func TestSearchHintNamesTagSearch(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	if got := SearchHint(s, cfg); got != "search by tag…" {
		t.Fatalf("root hint = %q, want search by tag…", got)
	}
	if !EnterFolder(s, []string{"clients"}, cfg, cols) {
		t.Fatal("clients folder not entered")
	}
	if got := SearchHint(s, cfg); got != "search by tag…" {
		t.Fatalf("hint in a tagged folder = %q, want search by tag…", got)
	}
}

func TestSearchHintWithoutTagSearch(t *testing.T) {
	s, cols, cfg := tagState(t, false)
	if got := SearchHint(s, cfg); got != "type to search…" {
		t.Fatalf("root hint with tag mode off = %q, want type to search…", got)
	}
	if !EnterFolder(s, []string{"clients"}, cfg, cols) {
		t.Fatal("clients folder not entered")
	}
	if got := SearchHint(s, cfg); got != "search…" {
		t.Fatalf("scoped hint with tag mode off = %q, want search…", got)
	}
}

// A folder whose rows carry no tags is searched by name even in tag mode, so
// its hint doesn't promise a tag search.
func TestSearchHintUntaggedFolderInTagMode(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	if !EnterFolder(s, []string{":"}, cfg, cols) {
		t.Fatal(": folder not entered")
	}
	if TagSearchReady(s, cfg) {
		t.Fatal("tag search ready in a folder with no tags")
	}
	if got := SearchHint(s, cfg); got != "search…" {
		t.Fatalf("hint = %q, want search…", got)
	}
	typeQuery(t, s, cfg, cols, "tags")
	if TagSearchActive(s.TopCtx()) {
		t.Fatal("typing in an untagged folder ran a tag search; the hint and the search disagree")
	}
}
