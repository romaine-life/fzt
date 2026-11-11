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
	if len(got) != 2 || got[0].Term != "move" || got[0].Tag != "move" || got[1].Term != "win" || got[1].Tag != "window" {
		t.Fatalf("TagTerms = %v, want move -> move and win -> window", got)
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

func TestTagSearchTabTargetIsTheShortestTag(t *testing.T) {
	tags := tagsStartingWith("pane", []string{"panes-extra", "Pane-b", "pane", "window"}, nil)
	if strings.Join(tags, ",") != "panes-extra,Pane-b,pane" {
		t.Fatalf("tags starting with pane = %v", tags)
	}
	if got := shortestTag(tags); got != "pane" {
		t.Fatalf("shortestTag = %q, want pane", got)
	}
}

// A row leaves the results only once what is typed rules it out: a word
// matches every row with a tag starting with it, so each added letter keeps a
// subset of what was shown, and a row whose name only holds the letters
// scattered ("Resize the pane left" for rel) never matches (FZT-80: re showed
// two rows, then rel showed a binding that wasn't among them).
func TestTagSearchTypingOnlyNarrows(t *testing.T) {
	items, err := LoadYAMLFromString(`
- name: tmux
  children:
    - name: claude-for-this-app
      tags: [claude, repo]
    - name: reload-app
      tags: [reload, restart]
    - name: claude-relaunch
      tags: [claude, relaunch]
    - name: Resize the pane left
      tags: [resize, pane]
    - name: kill-session
      tags: [kill, session]
`)
	if err != nil {
		t.Fatal(err)
	}
	var prev []string
	for _, step := range []struct {
		q    string
		want []string
	}{
		{"r", []string{"claude-for-this-app", "reload-app", "claude-relaunch", "Resize the pane left"}},
		{"re", []string{"claude-for-this-app", "reload-app", "claude-relaunch", "Resize the pane left"}},
		{"rel", []string{"reload-app", "claude-relaunch"}},
		{"relo", []string{"reload-app"}},
		{"reloz", nil},
	} {
		cfg := Config{Tiered: true, TagMode: true, DepthPenalty: 5}
		s, cols := NewState(items, cfg)
		s.TopCtx().TreeExpanded = make(map[int]bool)
		typeQuery(t, s, cfg, cols, step.q)
		got := filteredNames(s)
		if strings.Join(got, ",") != strings.Join(step.want, ",") {
			t.Fatalf("%q shows %v, want %v", step.q, got, step.want)
		}
		for _, name := range got {
			if prev != nil && !strings.Contains(","+strings.Join(prev, ",")+",", ","+name+",") {
				t.Fatalf("%q shows %s, which the shorter query didn't", step.q, name)
			}
		}
		prev = got
	}
}

// A finished word's tags are locked in: the next word doesn't match them
// again, so "session s" narrows by another tag starting with s, not by
// session itself.
func TestTagSearchFinishedWordsTagsAreLockedIn(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	typeQuery(t, s, cfg, cols, "session s")
	assertVisible(t, s, "switch-client")
	if got := s.TopCtx().TagTerms[1].Tags; strings.Join(got, ",") != "switch,switch-client" {
		t.Fatalf("s matched %v, want switch and switch-client, not session again", got)
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

func TestTagSearchSpaceStartsNextTagEvenOnAFolder(t *testing.T) {
	s, cols, cfg := tagState(t, true)
	// "windows" is a folder's name, so the folder is the top row; Space must
	// still type a space, not enter the folder.
	typeQuery(t, s, cfg, cols, "windows kill")
	if len(s.TopCtx().Scope) != 1 {
		t.Fatalf("Space entered a folder during a tag search")
	}
	if q := string(s.TopCtx().Query); q != "windows kill" {
		t.Fatalf("query = %q", q)
	}
}

func TestTagDefaultFolderSearchesByTagInside(t *testing.T) {
	s, cols, cfg := tagState(t, false)
	ctx := s.TopCtx()
	windows := -1
	for i, item := range ctx.AllItems {
		if item.Fields[0] == "windows" {
			windows = i
		}
	}
	ctx.AllItems[windows].TagDefault = true

	// Outside the folder the query is a name search, as with tag mode off.
	typeQuery(t, s, cfg, cols, "kill")
	if TagSearchOn(s) || TagSearchActive(s.TopCtx()) {
		t.Fatalf("root searched by tag: TagTerms %v", s.TopCtx().TagTerms)
	}
	for range "kill" {
		HandleSearchKeyResult(s, tcell.KeyBackspace2, 0, cfg, cols)
	}

	PushScope(s, windows, cfg, cols)
	if !TagSearchOn(s) {
		t.Fatal("inside a TagDefault folder, TagSearchOn is false")
	}
	typeQuery(t, s, cfg, cols, "kill")
	assertVisible(t, s, "kill-window")
	if got := s.TopCtx().TagTerms; len(got) != 1 || got[0].Tag != "kill" {
		t.Fatalf("TagTerms = %v, want kill resolved to kill", got)
	}
	if s.TagMode {
		t.Fatal("entering the folder turned tag mode on")
	}
}
