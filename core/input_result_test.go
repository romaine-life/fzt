package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func testTreeState(t *testing.T) (*State, []int, Config) {
	t.Helper()
	items, err := LoadYAMLFromString(`
- name: Example Links
  children:
    - name: Wikipedia
      url: https://en.wikipedia.org
    - name: Hacker News
      url: https://news.ycombinator.com
`)
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

func TestHandleUnifiedKeyResultReportsShiftEnterTopMatchItem(t *testing.T) {
	s, searchCols, cfg := testTreeState(t)

	for _, ch := range "hacker" {
		result := HandleUnifiedKeyResult(s, tcell.KeyRune, ch, false, cfg, searchCols)
		if result.Action != "" {
			t.Fatalf("typing emitted action %q", result.Action)
		}
	}

	result := HandleUnifiedKeyResult(s, tcell.KeyEnter, 0, true, cfg, searchCols)
	if result.Action != "select:Hacker News" {
		t.Fatalf("action = %q, want select:Hacker News", result.Action)
	}
	if !result.HasItem {
		t.Fatal("expected selected item")
	}
	if result.Item.Action == nil || result.Item.Action.Target != "https://news.ycombinator.com" {
		t.Fatalf("selected target = %#v, want Hacker News URL", result.Item.Action)
	}
}

func TestClickUnifiedRowResultReportsClickedItem(t *testing.T) {
	s, searchCols, cfg := testTreeState(t)
	_ = searchCols
	ctx := s.TopCtx()
	ctx.TreeExpanded[0] = true

	result := ClickUnifiedRowResult(s, 4, cfg, 24)
	if result.Action != "select:Wikipedia" {
		t.Fatalf("action = %q, want select:Wikipedia", result.Action)
	}
	if !result.HasItem {
		t.Fatal("expected selected item")
	}
	if result.Item.Action == nil || result.Item.Action.Target != "https://en.wikipedia.org" {
		t.Fatalf("selected target = %#v, want Wikipedia URL", result.Item.Action)
	}
}

func TestClickUnifiedRowWithoutPromptBar(t *testing.T) {
	s, _, cfg := testTreeState(t)
	cfg.HidePromptBar = true
	s.TopCtx().TreeExpanded[0] = true

	// No search box above the rows: row 0 is the folder, row 1 its first child.
	if result := ClickUnifiedRowResult(s, 1, cfg, 24); result.Action != "select:Wikipedia" {
		t.Fatalf("action = %q, want select:Wikipedia", result.Action)
	}
}

func TestPromptModeEmitsFrontendActionAndRestoresTreeState(t *testing.T) {
	s, searchCols, cfg := testTreeState(t)
	ctx := s.TopCtx()
	ctx.Query = []rune("wiki")
	ctx.Cursor = len(ctx.Query)
	ctx.SearchActive = true
	ctx.NavMode = false
	FilterItems(s, cfg, searchCols)
	SyncTreeCursorToTopMatch(s)
	beforeCursor := ctx.TreeCursor

	s.EnterPromptMode("google", "google-search", 'G', "search Google...")
	for _, ch := range "golang wasm" {
		result := HandleUnifiedKeyResult(s, tcell.KeyRune, ch, false, cfg, searchCols)
		if result.Action != "" {
			t.Fatalf("typing emitted action %q", result.Action)
		}
	}
	result := HandleUnifiedKeyResult(s, tcell.KeyEnter, 0, false, cfg, searchCols)
	if result.Action != "prompt:google-search:golang wasm" {
		t.Fatalf("action = %q, want google prompt action", result.Action)
	}
	if s.PromptMode != "" {
		t.Fatal("prompt mode still active after enter")
	}
	if string(ctx.Query) != "wiki" || ctx.TreeCursor != beforeCursor || !ctx.SearchActive || ctx.NavMode {
		t.Fatalf("tree state changed: query=%q cursor=%d search=%v nav=%v", string(ctx.Query), ctx.TreeCursor, ctx.SearchActive, ctx.NavMode)
	}
}

func TestPromptModeEscapeRestoresTreeState(t *testing.T) {
	s, searchCols, cfg := testTreeState(t)
	ctx := s.TopCtx()
	ctx.Query = []rune("hacker")
	ctx.Cursor = len(ctx.Query)
	ctx.SearchActive = true
	FilterItems(s, cfg, searchCols)
	SyncTreeCursorToTopMatch(s)
	beforeCursor := ctx.TreeCursor

	s.EnterPromptMode("google", "google-search", 'G', "search Google...")
	_ = HandleUnifiedKeyResult(s, tcell.KeyRune, 'x', false, cfg, searchCols)
	result := HandleUnifiedKeyResult(s, tcell.KeyEscape, 0, false, cfg, searchCols)
	if result.Action != "" {
		t.Fatalf("escape emitted action %q", result.Action)
	}
	if s.PromptMode != "" || len(s.PromptQuery) != 0 {
		t.Fatalf("prompt mode not cleared: mode=%q query=%q", s.PromptMode, string(s.PromptQuery))
	}
	if string(ctx.Query) != "hacker" || ctx.TreeCursor != beforeCursor || !ctx.SearchActive {
		t.Fatalf("tree state changed: query=%q cursor=%d search=%v", string(ctx.Query), ctx.TreeCursor, ctx.SearchActive)
	}
}

func TestPromptSelectionIsReplacedByTheFirstEdit(t *testing.T) {
	open := func() *State {
		s := &State{}
		s.EnterPromptMode("ask", "ask", 0, "")
		s.SetPromptText("take.wav", 4)
		return s
	}
	type key struct {
		key tcell.Key
		ch  rune
	}
	for _, c := range []struct {
		name         string
		keys         []key
		query        string
		cursor, left int
	}{
		{"typing replaces it", []key{{tcell.KeyRune, 'a'}, {tcell.KeyRune, 'b'}}, "ab.wav", 2, 0},
		{"backspace removes it", []key{{tcell.KeyBackspace2, 0}}, ".wav", 0, 0},
		{"delete removes it", []key{{tcell.KeyDelete, 0}}, ".wav", 0, 0},
		{"left goes to its start", []key{{tcell.KeyLeft, 0}}, "take.wav", 0, 0},
		{"right goes to its end", []key{{tcell.KeyRight, 0}}, "take.wav", 4, 0},
		{"home keeps the text", []key{{tcell.KeyHome, 0}}, "take.wav", 0, 0},
		{"end keeps the text", []key{{tcell.KeyEnd, 0}}, "take.wav", 8, 0},
		{"only the first key replaces", []key{{tcell.KeyEnd, 0}, {tcell.KeyRune, '!'}}, "take.wav!", 9, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := open()
			for _, k := range c.keys {
				HandlePromptModeKeyResult(s, k.key, k.ch)
			}
			if got := string(s.PromptQuery); got != c.query || s.PromptCursor != c.cursor || s.PromptSelected != c.left {
				t.Fatalf("query=%q cursor=%d selected=%d, want %q %d %d", got, s.PromptCursor, s.PromptSelected, c.query, c.cursor, c.left)
			}
		})
	}
}

func TestPromptEnterKeepsSelectedText(t *testing.T) {
	s := &State{}
	s.EnterPromptMode("ask", "ask", 0, "")
	s.SetPromptText("take.wav", 4)
	if got := HandlePromptModeKeyResult(s, tcell.KeyEnter, 0).Action; got != "prompt:ask:take.wav" {
		t.Fatalf("action = %q", got)
	}
}

func TestSetPromptTextKeepsTheSelectionInsideTheText(t *testing.T) {
	s := &State{}
	s.SetPromptText("ab", 9)
	if s.PromptSelected != 2 || s.PromptCursor != 2 {
		t.Fatalf("selected=%d cursor=%d", s.PromptSelected, s.PromptCursor)
	}
	s.SetPromptText("ab", -1)
	if s.PromptSelected != 0 || s.PromptCursor != 0 {
		t.Fatalf("selected=%d cursor=%d", s.PromptSelected, s.PromptCursor)
	}
}
