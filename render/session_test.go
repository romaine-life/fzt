package render

import (
	"testing"

	"github.com/romaine-life/fzt/core"
)

func TestPromptStateHintNamesTagSearch(t *testing.T) {
	items, err := core.LoadYAMLFromString(`
- name: new-session
- name: kill-window
`)
	if err != nil {
		t.Fatal(err)
	}
	items[0].Tags = []string{"session"}
	for _, tc := range []struct {
		tagMode bool
		want    string
	}{
		{true, "search by tag…"},
		{false, "type to search…"},
	} {
		sess := NewTreeSession(items, core.Config{Tiered: true, TagMode: tc.tagMode}, 80, 10, nil, nil)
		if got := sess.GetPromptState().Hint; got != tc.want {
			t.Errorf("tag mode %v: hint = %q, want %q", tc.tagMode, got, tc.want)
		}
	}
}
