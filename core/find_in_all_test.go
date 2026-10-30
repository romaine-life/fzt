package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Two folders each hold a row named life. Opening the second and searching
// it selects its own life, not the first folder's (FZT-56).
func TestSearchInAFolderSelectsItsOwnRow(t *testing.T) {
	items, err := LoadYAMLFromString(`
- name: choose-tree
  children:
    - name: default
    - name: life
- name: move-tab
  children:
    - name: life
    - name: system
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

	for _, ch := range "move-tab" {
		HandleUnifiedKey(s, tcell.KeyRune, ch, false, cfg, searchCols)
	}
	HandleUnifiedKey(s, tcell.KeyEnter, 0, false, cfg, searchCols)
	for _, ch := range "life" {
		HandleUnifiedKey(s, tcell.KeyRune, ch, false, cfg, searchCols)
	}

	visible := TreeVisibleItems(s)
	if ctx.TreeCursor < 0 {
		t.Fatal("nothing selected")
	}
	row := visible[ctx.TreeCursor]
	if parent := ctx.AllItems[row.Item.ParentIdx].Fields[0]; row.Item.Fields[0] != "life" || parent != "move-tab" {
		t.Fatalf("selected %q under %q, want life under move-tab", row.Item.Fields[0], parent)
	}
}
