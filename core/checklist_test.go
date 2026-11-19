package core

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

const checklistMenu = `
- name: senders
  description: Pick who to unsubscribe from
  checklist: true
  children:
    - name: news@a.com
      action: news@a.com
    - name: deals@b.com
      action: deals@b.com
    - name: news@c.com
      action: news@c.com
      ticked: true
- name: unsubscribe
  description: Unsubscribe from the ticked senders
  action: unsubscribe
- name: later
  action: later
`

func checklistState(t *testing.T) (*State, Config, []int) {
	t.Helper()
	items, err := LoadYAMLFromString(checklistMenu)
	if err != nil {
		t.Fatal(err)
	}
	cfg := treeCfg()
	s, cols := NewState(items, cfg)
	s.TopCtx().TreeExpanded = map[int]bool{}
	s.TopCtx().QueryExpanded = map[int]bool{}
	RestCursor(s, cfg)
	return s, cfg, cols
}

func pressAll(s *State, cfg Config, cols []int, ps ...keyPress) ActionResult {
	var res ActionResult
	for _, p := range ps {
		res = HandleUnifiedKeyResult(s, p.key, p.ch, p.shift, cfg, cols)
	}
	return res
}

func names(items []Item) string {
	var out []string
	for _, it := range items {
		out = append(out, it.Fields[0])
	}
	return strings.Join(out, ",")
}

// Enter opens the checklist, ticks its rows one after another, and a pick
// outside it answers with the ticked rows.
func TestChecklistTicksAndAPickCarriesThem(t *testing.T) {
	s, cfg, cols := checklistState(t)
	if res := pressAll(s, cfg, cols, keys(tcell.KeyEnter, tcell.KeyEnter)...); res.Action != "" {
		t.Fatalf("opening the folder and ticking answered %q", res.Action)
	}
	if got := names(Ticked(s)); got != "news@a.com,news@c.com" {
		t.Fatalf("ticked %s", got)
	}
	res := pressAll(s, cfg, cols, seq(keys(tcell.KeyEscape), runes("unsub"), keys(tcell.KeyEnter))...)
	if res.Action != "select:unsubscribe\tUnsubscribe from the ticked senders\nnews@a.com\nnews@c.com" || names(res.Items) != "news@a.com,news@c.com" {
		t.Fatalf("pick: %q items %s", res.Action, names(res.Items))
	}
	if res.Item.Fields[0] != "unsubscribe" {
		t.Fatalf("picked %v", res.Item.Fields)
	}
}

func TestChecklistEnterUnticks(t *testing.T) {
	s, cfg, cols := checklistState(t)
	pressAll(s, cfg, cols, keys(tcell.KeyEnter, tcell.KeyDown, tcell.KeyDown, tcell.KeyEnter)...) // into the folder, down to news@c (pre-ticked)
	if got := names(Ticked(s)); got != "" {
		t.Fatalf("ticked %s, want news@c unticked", got)
	}
}

// Searching from the top finds checklist rows, and Enter ticks down the
// matches with the search kept.
func TestChecklistTicksMatchesInARow(t *testing.T) {
	s, cfg, cols := checklistState(t)
	pressAll(s, cfg, cols, seq(runes("news"), keys(tcell.KeyEnter, tcell.KeyEnter))...)
	if q := string(s.TopCtx().Query); q != "news" {
		t.Fatalf("query %q, want the search kept", q)
	}
	if got := names(Ticked(s)); got != "news@a.com" {
		t.Fatalf("ticked %s (news@c was ticked, so the second Enter unticks it)", got)
	}
}

// Shift+Enter, which picks a row anywhere else, ticks a checklist row too.
func TestChecklistShiftEnterTicks(t *testing.T) {
	s, cfg, cols := checklistState(t)
	pressAll(s, cfg, cols, keys(tcell.KeyEnter)...)
	if res := pressAll(s, cfg, cols, keyPress{key: tcell.KeyEnter, shift: true}); res.Action != "" {
		t.Fatalf("Shift+Enter answered %q", res.Action)
	}
	if got := names(Ticked(s)); got != "news@a.com,news@c.com" {
		t.Fatalf("ticked %s", got)
	}
}

func TestChecklistClickTicks(t *testing.T) {
	s, cfg, cols := checklistState(t)
	pressAll(s, cfg, cols, keys(tcell.KeyEnter)...)
	row := firstItemRow(s.TopCtx(), cfg) - s.TopCtx().TreeOffset + 2 // deals@b, under senders and news@a
	if res := ClickUnifiedRowResult(s, row, cfg, 20); res.Action != "" {
		t.Fatalf("a click answered: %q", res.Action)
	}
	if got := names(Ticked(s)); got != "deals@b.com,news@c.com" {
		t.Fatalf("ticked %s", got)
	}
}

// A menu with no checklist picks as it always has.
func TestNoChecklistPickUnchanged(t *testing.T) {
	items, err := LoadYAMLFromString(keysTestMenu)
	if err != nil {
		t.Fatal(err)
	}
	cfg := treeCfg()
	s, cols := NewState(items, cfg)
	s.TopCtx().TreeExpanded = map[int]bool{}
	s.TopCtx().QueryExpanded = map[int]bool{}
	if res := pressAll(s, cfg, cols, seq(runes("lights"), keys(tcell.KeyEnter))...); res.Action != "select:lights" || res.Items != nil {
		t.Fatalf("got %+v", res)
	}
}

func TestChecklistPopupListsTick(t *testing.T) {
	s, cfg, cols := checklistState(t)
	pressAll(s, cfg, cols, seq(keys(tcell.KeyEnter), []keyPress{{key: tcell.KeyRune, ch: '/'}})...)
	tick := rowFor(t, s, "tick")
	if tick.Desc != "Tick news@a.com" || tick.Keys[0] != "Enter" {
		t.Fatalf("tick row %+v", tick)
	}
}
