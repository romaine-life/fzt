package core

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

const keysTestMenu = `
- name: git
  description: Branch and repo management
  children:
    - name: gitprune
      action: gitprune
    - name: gitsub
      action: gitsub
- name: workspace
  description: Manage workspace entries
  children:
    - name: wsopen
      description: Open a workspace in VS Code
      action: wsopen
    - name: wsload
      action: wsload
    - name: wsorg
      action: wsorg
- name: lights
  action: lights
`

const keysTestTagged = `
- name: session
  action: session
  tags: [tmux, window]
- name: split
  action: split
  tags: [tmux, pane]
`

type keyFixture struct {
	name  string
	yaml  string
	cfg   Config
	flat  bool
	steps []keyPress // replayed from a fresh state
}

func runes(text string) []keyPress {
	var out []keyPress
	for _, r := range text {
		out = append(out, keyPress{key: tcell.KeyRune, ch: r})
	}
	return out
}

func keys(k ...tcell.Key) []keyPress {
	var out []keyPress
	for _, key := range k {
		out = append(out, keyPress{key: key})
	}
	return out
}

func seq(parts ...[]keyPress) []keyPress {
	var out []keyPress
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func treeCfg() Config { return Config{Tiered: true, Nth: []int{1}, DepthPenalty: 5} }

func newKeysState(t *testing.T, f keyFixture) (*State, []int) {
	t.Helper()
	var items []Item
	if f.flat {
		items = ParseLines([]string{"wsopen", "wsload", "wsorg", "lights", "gitprune"}, "\t", false, false)
	} else {
		var err error
		items, err = LoadYAMLFromString(f.yaml)
		if err != nil {
			t.Fatal(err)
		}
	}
	s, cols := NewState(items, f.cfg)
	ctx := s.TopCtx()
	ctx.TreeExpanded = map[int]bool{}
	ctx.QueryExpanded = map[int]bool{}
	ctx.TreeCursor = -1
	for _, p := range f.steps {
		pressKey(s, f, p, cols)
	}
	return s, cols
}

func pressKey(s *State, f keyFixture, p keyPress, cols []int) ActionResult {
	if p.key == tcell.KeyBackspace2 && p.shift {
		GoHome(s, f.cfg, cols) // fzt-terminal's Shift+Backspace
		return ActionResult{}
	}
	if f.flat {
		return HandleKeyEventResult(s, p.key, p.ch, p.shift, f.cfg, cols)
	}
	return HandleUnifiedKeyResult(s, p.key, p.ch, p.shift, f.cfg, cols)
}

// snapshot is everything a key can change that the person at the screen
// would notice.
func snapshot(s *State, res ActionResult) string {
	ctx := s.TopCtx()
	var scope []int
	for _, l := range ctx.Scope {
		scope = append(scope, l.ParentIdx)
	}
	var open []int
	for i, on := range ctx.TreeExpanded {
		if on {
			open = append(open, i)
		}
	}
	sort.Ints(open)
	return fmt.Sprintf("q=%q c=%d tc=%d nav=%v search=%v scope=%v open=%v ctx=%d idx=%d act=%q cancel=%v",
		string(ctx.Query), ctx.Cursor, ctx.TreeCursor, ctx.NavMode, ctx.SearchActive, scope, open,
		len(s.Contexts), ctx.Index, res.Action, s.Cancelled)
}

func keyFixtures() []keyFixture {
	tree := treeCfg()
	picker := treeCfg()
	picker.ScopedView, picker.FoldersOnly = true, true
	tagged := treeCfg()
	tagged.TagMode = true
	flat := Config{Nth: []int{1}}
	down, up, tab, esc, enter, right := keys(tcell.KeyDown), keys(tcell.KeyUp), keys(tcell.KeyTab), keys(tcell.KeyEscape), keys(tcell.KeyEnter), keys(tcell.KeyRight)
	return []keyFixture{
		{name: "opening", yaml: keysTestMenu, cfg: tree},
		{name: "typed wso", yaml: keysTestMenu, cfg: tree, steps: runes("wso")},
		{name: "typed exact name", yaml: keysTestMenu, cfg: tree, steps: runes("wsopen")},
		{name: "tab completed", yaml: keysTestMenu, cfg: tree, steps: seq(runes("wso"), tab)},
		{name: "tab stepped", yaml: keysTestMenu, cfg: tree, steps: seq(runes("wso"), tab, tab)},
		{name: "folder on top", yaml: keysTestMenu, cfg: tree, steps: runes("work")},
		{name: "no match", yaml: keysTestMenu, cfg: tree, steps: runes("zzz")},
		{name: "normal mode from search", yaml: keysTestMenu, cfg: tree, steps: seq(runes("wso"), esc)},
		{name: "normal mode on folder", yaml: keysTestMenu, cfg: tree, steps: down},
		{name: "normal mode on leaf", yaml: keysTestMenu, cfg: tree, steps: seq(up)},
		{name: "folder expanded", yaml: keysTestMenu, cfg: tree, steps: seq(down, right)},
		{name: "inside folder", yaml: keysTestMenu, cfg: tree, steps: seq(runes("workspace"), runes(" "))},
		{name: "search inside folder", yaml: keysTestMenu, cfg: tree, steps: seq(runes("workspace"), runes(" "), runes("wsl"))},
		{name: "walked in search", yaml: keysTestMenu, cfg: tree, steps: seq(runes("ws"), down, down)},
		{name: "picker opening", yaml: keysTestMenu, cfg: picker},
		{name: "picker typed", yaml: keysTestMenu, cfg: picker, steps: runes("wo")},
		{name: "picker inside", yaml: keysTestMenu, cfg: picker, steps: seq(down, enter)},
		{name: "tag search", yaml: keysTestTagged, cfg: tagged, steps: runes("tm")},
		{name: "flat opening", cfg: flat, flat: true},
		{name: "flat typed", cfg: flat, flat: true, steps: runes("ws")},
		{name: "flat walked", cfg: flat, flat: true, steps: seq(runes("ws"), down)},
		{name: "flat text cursor", cfg: flat, flat: true, steps: seq(runes("ws"), keys(tcell.KeyLeft, tcell.KeyLeft))},
	}
}

// TestKeyActionsListExactlyWhatActs is the pop-up's contract: in every
// fixture, a key appears in KeyActions exactly when pressing it changes
// something.
func TestKeyActionsListExactlyWhatActs(t *testing.T) {
	for _, f := range keyFixtures() {
		t.Run(f.name, func(t *testing.T) {
			s, _ := newKeysState(t, f)
			listed := map[string]string{}
			for _, a := range KeyActions(s, f.cfg, f.flat) {
				for _, p := range a.press {
					listed[pressName(p)] = a.Verb + ": " + a.Desc
				}
			}
			for _, p := range keyPresses {
				if f.flat && p.key == tcell.KeyBackspace2 && p.shift {
					continue // fzt-terminal's tree key
				}
				s, cols := newKeysState(t, f)
				before := snapshot(s, ActionResult{})
				after := snapshot(s, pressKey(s, f, p, cols))
				changed := before != after
				row, isListed := listed[pressName(p)]
				if changed && !isListed {
					t.Errorf("%s changes state but isn't listed\n before %s\n after  %s", pressName(p), before, after)
				}
				if !changed && isListed {
					t.Errorf("%s is listed as %q but changes nothing (%s)", pressName(p), row, before)
				}
			}
		})
	}
}

func pressName(p keyPress) string {
	if p.key == tcell.KeyRune {
		return fmt.Sprintf("%q", p.ch)
	}
	if p.shift {
		return "Shift+" + tcell.KeyNames[p.key]
	}
	return tcell.KeyNames[p.key]
}

func rowFor(t *testing.T, s *State, verb string) KeyAction {
	t.Helper()
	ctx := s.TopCtx()
	for _, a := range ctx.keyActions {
		if a.Verb == verb {
			return a
		}
	}
	t.Fatalf("no %q row in the pop-up: %v", verb, ctx.keyActions)
	return KeyAction{}
}

func TestSlashOpensPopupOverAnyMode(t *testing.T) {
	for _, steps := range [][]keyPress{nil, runes("wso"), seq(runes("wso"), keys(tcell.KeyEscape)), keys(tcell.KeyDown)} {
		f := keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: steps}
		s, cols := newKeysState(t, f)
		before := snapshot(s, ActionResult{})
		pressKey(s, f, keyPress{key: tcell.KeyRune, ch: '/'}, cols)
		if !KeyPopupOpen(s) {
			t.Fatalf("after %v, / didn't open the pop-up", steps)
		}
		pressKey(s, f, keyPress{key: tcell.KeyRune, ch: '/'}, cols)
		if KeyPopupOpen(s) {
			t.Fatal("a second / didn't close the pop-up")
		}
		if after := snapshot(s, ActionResult{}); after != before {
			t.Fatalf("opening and closing the pop-up changed the menu\n before %s\n after  %s", before, after)
		}
	}
}

func TestPopupClosesOnEscAndKeepsTheSearch(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: runes("wso")}
	s, cols := newKeysState(t, f)
	pressKey(s, f, keyPress{key: tcell.KeyRune, ch: '/'}, cols)
	pressKey(s, f, keyPress{key: tcell.KeyEscape}, cols)
	if KeyPopupOpen(s) {
		t.Fatal("Esc didn't close the pop-up")
	}
	if q := string(s.TopCtx().Query); q != "wso" {
		t.Fatalf("query after closing the pop-up = %q, want wso", q)
	}
}

func TestPopupRowsShowKeysAndRunInTheMenu(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: seq(runes("wso"), keys(tcell.KeyTab))}
	s, cols := newKeysState(t, f)
	pressKey(s, f, keyPress{key: tcell.KeyRune, ch: '/'}, cols)
	if KeyPopupTitle(s) != "wsopen" {
		t.Fatalf("pop-up acts on %q, want wsopen", KeyPopupTitle(s))
	}
	run := rowFor(t, s, "run")
	if run.Desc != "Run wsopen" || strings.Join(run.Keys, " ") != "Enter Shift+Enter" {
		t.Fatalf("run row = %q %v", run.Desc, run.Keys)
	}
	// Pick "next match" by typing its verb, as the person would.
	for _, p := range runes("next") {
		pressKey(s, f, p, cols)
	}
	res := pressKey(s, f, keyPress{key: tcell.KeyEnter}, cols)
	if KeyPopupOpen(s) || res.Action != "" {
		t.Fatalf("picking next match left the pop-up open or returned %q", res.Action)
	}
	ctx := s.TopCtx()
	if got := itemName(ctx.Filtered[0]); got == "wsopen" || !strings.HasPrefix(got, "ws") {
		t.Fatalf("next match from the pop-up landed on %q", got)
	}
}

func TestPopupRunRowSelectsTheMatch(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: runes("wsopen")}
	s, cols := newKeysState(t, f)
	pressKey(s, f, keyPress{key: tcell.KeyRune, ch: '/'}, cols)
	res := pressKey(s, f, keyPress{key: tcell.KeyEnter}, cols) // first row: run
	if !res.HasItem || itemName(res.Item) != "wsopen" {
		t.Fatalf("Enter on the pop-up's run row returned %+v", res)
	}
}

func TestPopupClickRunsTheRow(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: runes("wsopen")}
	s, cols := newKeysState(t, f)
	pressKey(s, f, keyPress{key: tcell.KeyRune, ch: '/'}, cols)
	res := ClickKeyPopupRow(s, 0, f.cfg)
	if !res.HasItem || itemName(res.Item) != "wsopen" {
		t.Fatalf("clicking the pop-up's run row returned %+v", res)
	}
}

func TestPopupSearchRowLeavesNormalMode(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: seq(runes("wso"), keys(tcell.KeyEscape))}
	s, cols := newKeysState(t, f)
	pressKey(s, f, keyPress{key: tcell.KeyRune, ch: '/'}, cols)
	a := rowFor(t, s, "search")
	if len(a.Keys) != 0 || a.Desc != "Go back to typing" {
		t.Fatalf("search row = %q %v, want a row with no key", a.Desc, a.Keys)
	}
	q := string(s.Contexts[0].Query)
	for _, p := range runes("search") {
		pressKey(s, f, p, cols)
	}
	pressKey(s, f, keyPress{key: tcell.KeyEnter}, cols)
	ctx := s.TopCtx()
	if KeyPopupOpen(s) || ctx.NavMode || string(ctx.Query) != q {
		t.Fatalf("after the search row: popup=%v nav=%v query=%q (want %q)", KeyPopupOpen(s), ctx.NavMode, string(ctx.Query), q)
	}
}

func TestSlashStaysTextInAPromptBox(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg()}
	s, cols := newKeysState(t, f)
	s.EnterPromptMode("ask", "branch", 0, "")
	pressKey(s, f, keyPress{key: tcell.KeyRune, ch: '/'}, cols)
	if KeyPopupOpen(s) || string(s.PromptQuery) != "/" {
		t.Fatalf("/ in a prompt box: popup=%v text=%q", KeyPopupOpen(s), string(s.PromptQuery))
	}
}

func TestTabStepsThroughTheTypedRanking(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: runes("wso")}
	s, cols := newKeysState(t, f)
	ranking := []string{}
	for _, it := range s.TopCtx().Filtered {
		ranking = append(ranking, itemName(it))
	}
	if len(ranking) < 2 {
		t.Fatalf("fixture needs two matches for wso, got %v", ranking)
	}
	tab := func(k tcell.Key) string {
		pressKey(s, f, keyPress{key: k}, cols)
		ctx := s.TopCtx()
		visible := TreeVisibleItems(s)
		if ctx.TreeCursor < 0 || itemName(visible[ctx.TreeCursor].Item) != string(ctx.Query) {
			t.Fatalf("cursor isn't on the stepped-to match %q", string(ctx.Query))
		}
		return string(ctx.Query)
	}
	if got := tab(tcell.KeyTab); got != ranking[0] {
		t.Fatalf("first Tab = %q, want %q", got, ranking[0])
	}
	if got := tab(tcell.KeyTab); got != ranking[1] {
		t.Fatalf("second Tab = %q, want %q", got, ranking[1])
	}
	if got := tab(tcell.KeyBacktab); got != ranking[0] {
		t.Fatalf("Shift+Tab = %q, want %q", got, ranking[0])
	}
	if got := tab(tcell.KeyBacktab); got != ranking[len(ranking)-1] {
		t.Fatalf("Shift+Tab past the first = %q, want the last, %q", got, ranking[len(ranking)-1])
	}
	pressKey(s, f, keyPress{key: tcell.KeyBackspace2}, cols)
	if s.TopCtx().tabRanking != nil {
		t.Fatal("typing kept the old Tab ranking")
	}
}

func TestTabLocksInAFolderWithoutOpeningIt(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg(), steps: runes("work")}
	s, cols := newKeysState(t, f)
	pressKey(s, f, keyPress{key: tcell.KeyTab}, cols)
	ctx := s.TopCtx()
	if len(ctx.Scope) != 1 || string(ctx.Query) != "workspace" {
		t.Fatalf("Tab on a folder: scope depth %d, query %q; want it locked in, not opened", len(ctx.Scope), string(ctx.Query))
	}
}

func TestTabOpensAFolderInAFolderPicker(t *testing.T) {
	cfg := treeCfg()
	cfg.ScopedView, cfg.FoldersOnly = true, true
	f := keyFixture{yaml: keysTestMenu, cfg: cfg, steps: runes("work")}
	s, cols := newKeysState(t, f)
	pressKey(s, f, keyPress{key: tcell.KeyTab}, cols)
	if len(s.TopCtx().Scope) != 2 {
		t.Fatal("Tab in a folder picker didn't go into the folder (FZT-51)")
	}
}

func TestFlatPopupRunsTheHighlightedLine(t *testing.T) {
	f := keyFixture{cfg: Config{Nth: []int{1}}, flat: true, steps: runes("wsl")}
	s, cols := newKeysState(t, f)
	pressKey(s, f, keyPress{key: tcell.KeyRune, ch: '/'}, cols)
	if !KeyPopupOpen(s) || string(s.Contexts[0].Query) != "wsl" {
		t.Fatal("/ in a flat list didn't open the pop-up over the query")
	}
	res := pressKey(s, f, keyPress{key: tcell.KeyEnter}, cols)
	if !res.HasItem || itemName(res.Item) != "wsload" {
		t.Fatalf("the flat pop-up's run row returned %+v", res)
	}
}
