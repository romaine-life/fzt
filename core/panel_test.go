package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

const quickYAML = `
- name: lights
  children:
    - name: lightson
      action: lightson
      quick: true
    - name: ollamastop
      action: ollamastop
      quick: true
      input: model to stop
    - name: spt
      action: spt
`

func TestQuickAndInputLoadAndSurviveSave(t *testing.T) {
	items, err := LoadYAMLFromString(quickYAML)
	if err != nil {
		t.Fatal(err)
	}
	if !items[1].Quick || items[1].Input != "" {
		t.Fatalf("lightson = quick %v input %q", items[1].Quick, items[1].Input)
	}
	if !items[2].Quick || items[2].Input != "model to stop" {
		t.Fatalf("ollamastop = quick %v input %q", items[2].Quick, items[2].Input)
	}
	if items[3].Quick {
		t.Fatal("spt has no quick key and must not be quick")
	}

	s, _ := NewState(items, Config{})
	tree := SerializeTree(s.TopCtx())
	children := tree[0].(map[string]interface{})["children"].([]interface{})
	on := children[0].(map[string]interface{})
	stop := children[1].(map[string]interface{})
	spt := children[2].(map[string]interface{})
	if on["quick"] != true || stop["input"] != "model to stop" {
		t.Fatalf("save dropped the fields: %v %v", on, stop)
	}
	if _, ok := spt["quick"]; ok {
		t.Fatalf("save wrote quick on a takeover entry: %v", spt)
	}
	if _, ok := spt["input"]; ok {
		t.Fatalf("save wrote an empty input: %v", spt)
	}
}

func TestSetPanelKeepsThreeLinesAndNotesTheCut(t *testing.T) {
	s := &State{}
	s.SetPanel("one\r\n\ntwo\nthree  \nfour\n", PanelOK)
	if got := s.Panel.Lines; len(got) != 3 || got[0] != "one" || got[2] != "three" {
		t.Fatalf("lines = %q", got)
	}
	if !s.Panel.Clipped || s.Panel.Style != PanelOK {
		t.Fatalf("panel = %+v", s.Panel)
	}
	s.SetPanel("done", PanelFailed)
	if s.Panel.Clipped || len(s.Panel.Lines) != 1 {
		t.Fatalf("a new result must replace the last: %+v", s.Panel)
	}
	s.ClearPanel()
	if len(s.Panel.Lines) != 0 {
		t.Fatal("ClearPanel left lines")
	}
}

func TestFrozenDropsKeysAndClicks(t *testing.T) {
	s, searchCols, cfg := testTreeState(t)
	s.Freeze()
	for _, ch := range "hacker" {
		HandleUnifiedKeyResult(s, tcell.KeyRune, ch, false, cfg, searchCols)
	}
	if r := HandleUnifiedKeyResult(s, tcell.KeyEnter, 0, false, cfg, searchCols); r.Action != "" {
		t.Fatalf("frozen Enter emitted %q", r.Action)
	}
	if q := string(s.TopCtx().Query); q != "" {
		t.Fatalf("frozen typing changed the query to %q", q)
	}
	if r := ClickUnifiedRowResult(s, 4, cfg, 20); r.Action != "" {
		t.Fatalf("frozen click emitted %q", r.Action)
	}
	s.Unfreeze()
	for _, ch := range "hacker" {
		HandleUnifiedKeyResult(s, tcell.KeyRune, ch, false, cfg, searchCols)
	}
	if q := string(s.TopCtx().Query); q != "hacker" {
		t.Fatalf("after Unfreeze the query is %q", q)
	}
}

func TestClickRowsShiftByThePanel(t *testing.T) {
	s, _, cfg := testTreeState(t)
	// No border, prompt bar 3 rows: the first row is at 3, or at 3+PanelRows with the panel.
	if r := ClickUnifiedRowResult(s, 3, cfg, 20); r.Action != "" {
		t.Fatalf("row 3 (a folder) emitted %q", r.Action)
	}
	if s.TopCtx().TreeCursor != 0 {
		t.Fatalf("without the panel row 3 should be the first row, cursor %d", s.TopCtx().TreeCursor)
	}
	cfg.ShowPanel = true
	s.TopCtx().TreeCursor = -1
	ClickUnifiedRowResult(s, 3, cfg, 20)
	if s.TopCtx().TreeCursor != -1 {
		t.Fatalf("with the panel row 3 is a panel row, cursor moved to %d", s.TopCtx().TreeCursor)
	}
	ClickUnifiedRowResult(s, 3+PanelRows, cfg, 20)
	if s.TopCtx().TreeCursor != 0 {
		t.Fatalf("with the panel the first row is %d, cursor %d", 3+PanelRows, s.TopCtx().TreeCursor)
	}
}

func TestInspectEditsQuick(t *testing.T) {
	items, _ := LoadYAMLFromString(quickYAML)
	s, _ := NewState(items, Config{})
	ctx := s.TopCtx()
	ctx.AllItems = append(ctx.AllItems, Item{Fields: []string{"quick", "false"}, IsProperty: true, PropertyOf: 3, PropertyKey: "quick"})
	prop := len(ctx.AllItems) - 1
	edit := func(v string) {
		s.EditMode, s.EditTargetIdx, s.EditBuffer = "rename", prop, []rune(v)
		handleRenameKey(s, tcell.KeyEnter, 0)
	}
	edit("maybe")
	if ctx.AllItems[3].Quick || s.EditMode != "rename" {
		t.Fatalf("a bad value must be refused and keep the editor open: quick %v mode %q", ctx.AllItems[3].Quick, s.EditMode)
	}
	edit("Yes")
	if !ctx.AllItems[3].Quick || ctx.AllItems[prop].Fields[1] != "true" || s.EditMode != "" {
		t.Fatalf("quick %v shown %q mode %q", ctx.AllItems[3].Quick, ctx.AllItems[prop].Fields[1], s.EditMode)
	}
}
