package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// A folder opened in place (the tree view, not the scoped one) becomes the
// list's first row, so the rows it opened are on screen even when it was the
// last row showing (FZT-55).
func TestOpeningAFolderScrollsItToTheTop(t *testing.T) {
	var yaml strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&yaml, "- name: cmd-%02d\n", i)
	}
	yaml.WriteString("- name: move-tab\n  children:\n    - name: session-a\n    - name: session-b\n")
	items, err := LoadYAMLFromString(yaml.String())
	if err != nil {
		t.Fatalf("LoadYAMLFromString: %v", err)
	}

	for _, tc := range []struct {
		name string
		keys []tcell.Key
	}{
		{"Enter", []tcell.Key{tcell.KeyEnter}},
		{"Right", []tcell.Key{tcell.KeyRight}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Tiered: true}
			s, searchCols := NewState(items, cfg)
			ctx := s.TopCtx()
			ctx.TreeExpanded = make(map[int]bool)
			ctx.QueryExpanded = make(map[int]bool)
			ctx.TreeCursor = 30
			ctx.NavMode = true
			ctx.TreeOffset = 20

			for _, k := range tc.keys {
				HandleUnifiedKey(s, k, 0, false, cfg, searchCols)
			}

			visible := TreeVisibleItems(s)
			if ctx.TreeOffset != 30 || ctx.TreeCursor != 30 {
				t.Fatalf("offset %d, cursor %d; want the folder's row, 30", ctx.TreeOffset, ctx.TreeCursor)
			}
			if got := visible[ctx.TreeOffset+1].Item.Fields[0]; got != "session-a" {
				t.Fatalf("row after the folder is %q, want session-a", got)
			}
		})
	}
}
