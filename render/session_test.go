package render

import (
	"testing"

	"github.com/romaine-life/fzt/core"
)

// Tag mode shows as fzt-terminal's prompt icon, not in the hint text (FZT-70).
func TestPromptStateHintIgnoresTagMode(t *testing.T) {
	items, err := core.LoadYAMLFromString(`
- name: new-session
  tags: [session]
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tagMode := range []bool{true, false} {
		sess := NewTreeSession(items, core.Config{Tiered: true, TagMode: tagMode}, 80, 10, nil, nil)
		if got := sess.GetPromptState().Hint; got != "type to search…" {
			t.Errorf("tag mode %v: hint = %q, want type to search…", tagMode, got)
		}
	}
}
