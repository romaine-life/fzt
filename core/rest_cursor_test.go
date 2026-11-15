package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// With nothing typed, Enter has nothing else to do, so every list rests on its first row
// and Enter at once acts on it (FZTA-21).

func TestAFlatListEnterAtOncePicksTheFirstRow(t *testing.T) {
	cfg := Config{HeaderLines: 1, Delimiter: "\t"}
	s, searchCols := NewState(ParseLines([]string{"Repo\tabout", "D:\\repos\tthe folder", "alpha\ta repo"}, "\t", false, false), cfg)
	result := HandleKeyEventResult(s, tcell.KeyEnter, 0, false, cfg, searchCols)
	if !result.HasItem || result.Item.Fields[0] != "D:\\repos" {
		t.Fatalf("Enter at once picked %v (action %q), want the D:\\repos row", result.Item.Fields, result.Action)
	}
}

func restTree(t *testing.T) (*State, []int, Config) {
	t.Helper()
	items, err := LoadYAMLFromString(`
- name: Wikipedia
  url: https://en.wikipedia.org
- name: Hacker News
  url: https://news.ycombinator.com
`)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Tiered: true}
	s, searchCols := NewState(items, cfg)
	ctx := s.TopCtx()
	ctx.TreeExpanded = map[int]bool{}
	ctx.QueryExpanded = map[int]bool{}
	RestCursor(s, cfg)
	return s, searchCols, cfg
}

func TestTheTreeAtRestEnterAtOncePicksTheFirstRow(t *testing.T) {
	s, searchCols, cfg := restTree(t)
	if r := HandleUnifiedKeyResult(s, tcell.KeyEnter, 0, false, cfg, searchCols); r.Action != "select:Wikipedia" {
		t.Fatalf("Enter at once = %q, want select:Wikipedia", r.Action)
	}
}

func TestErasingTheQueryReturnsTheTreeToRest(t *testing.T) {
	for name, clear := range map[string][]tcell.Key{
		"Backspace": {tcell.KeyBackspace2, tcell.KeyBackspace2},
		"Escape":    {tcell.KeyEscape, tcell.KeyEscape},
	} {
		s, searchCols, cfg := restTree(t)
		for _, ch := range "ha" {
			HandleUnifiedKeyResult(s, tcell.KeyRune, ch, false, cfg, searchCols)
		}
		for _, k := range clear {
			HandleUnifiedKeyResult(s, k, 0, false, cfg, searchCols)
		}
		if r := HandleUnifiedKeyResult(s, tcell.KeyEnter, 0, false, cfg, searchCols); r.Action != "select:Wikipedia" {
			t.Errorf("%s: Enter after the query was erased = %q, want select:Wikipedia", name, r.Action)
		}
	}
	s, searchCols, cfg := restTree(t)
	HandleUnifiedKeyResult(s, tcell.KeyDown, 0, false, cfg, searchCols)
	GoHome(s, cfg, searchCols)
	if got := s.TopCtx().TreeCursor; got != 0 {
		t.Fatalf("cursor = %d after going home, want 0", got)
	}
}

// A folder picker's Enter at rest picks the folder you are in, so it rests on nothing
// (FZT-51).
func TestAFolderPickerRestsOnNothing(t *testing.T) {
	s, _, _ := scopedState(t, true)
	if got := s.TopCtx().TreeCursor; got != -1 {
		t.Fatalf("cursor = %d at rest, want -1", got)
	}
}
