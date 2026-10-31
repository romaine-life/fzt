package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// windowsProvider stands in for fzt-automate's open-windows folder: each load
// returns whatever windows are open now.
type windowsProvider struct {
	open     []string
	loads    int
	volatile bool
}

func (p *windowsProvider) LoadChildren(string) []Item {
	p.loads++
	var rows []Item
	for _, w := range p.open {
		rows = append(rows, Item{Fields: []string{w}, PropertyOf: -1})
	}
	return rows
}

func (p *windowsProvider) Volatile(string) bool { return p.volatile }

type plainProvider struct{ *windowsProvider }

func (p plainProvider) LoadChildren(path string) []Item { return p.windowsProvider.LoadChildren(path) }

func enterWindows(t *testing.T, s *State, cfg Config, searchCols []int) []string {
	t.Helper()
	PushScope(s, 0, cfg, searchCols)
	var names []string
	for _, row := range TreeVisibleItems(s) {
		if row.Item.Depth == 1 {
			names = append(names, row.Item.Fields[0])
		}
	}
	PopScope(s, cfg, searchCols)
	return names
}

func newWindowsState(t *testing.T, p TreeProvider) (*State, Config, []int) {
	t.Helper()
	items := []Item{{Fields: []string{"windows"}, HasChildren: true, ParentIdx: -1, PropertyOf: -1}}
	cfg := Config{Tiered: true}
	s, searchCols := NewState(items, cfg)
	s.Provider = p
	ctx := s.TopCtx()
	ctx.TreeExpanded = map[int]bool{}
	ctx.QueryExpanded = map[int]bool{}
	ctx.TreeCursor = -1
	return s, cfg, searchCols
}

func TestVolatileFolderReloadsOnEveryEntry(t *testing.T) {
	p := &windowsProvider{open: []string{"Chrome", "Discord"}, volatile: true}
	s, cfg, searchCols := newWindowsState(t, p)
	if got := enterWindows(t, s, cfg, searchCols); len(got) != 2 {
		t.Fatalf("first entry = %v", got)
	}
	p.open = []string{"Chrome", "Steam"}
	got := enterWindows(t, s, cfg, searchCols)
	if p.loads != 2 || len(got) != 2 || got[1] != "Steam" {
		t.Fatalf("second entry = %v after %d loads, want the windows open now", got, p.loads)
	}
	// A closed window's old row is gone from search too.
	for _, ch := range "discord" {
		HandleUnifiedKeyResult(s, tcell.KeyRune, ch, false, cfg, searchCols)
	}
	for _, it := range s.TopCtx().Filtered {
		if it.Fields[0] == "Discord" {
			t.Fatal("search still finds a row from an earlier load")
		}
	}
}

func TestPlainProviderLoadsOnce(t *testing.T) {
	p := &windowsProvider{open: []string{"Chrome"}}
	s, cfg, searchCols := newWindowsState(t, plainProvider{p})
	enterWindows(t, s, cfg, searchCols)
	enterWindows(t, s, cfg, searchCols)
	if p.loads != 1 {
		t.Fatalf("a provider that isn't volatile loaded %d times", p.loads)
	}
}
