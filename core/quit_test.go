package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Ctrl+C closes the menu in one press from wherever the user is, where
// Escape would step back one level at a time.
func TestCtrlCQuitsFromAnyDepth(t *testing.T) {
	ctrlC := keyPress{key: tcell.KeyCtrlC}
	for _, f := range []keyFixture{
		{name: "top", yaml: keysTestMenu, cfg: treeCfg()},
		{name: "search inside folder", yaml: keysTestMenu, cfg: treeCfg(), steps: seq(runes("workspace"), runes(" "), runes("wsl"))},
		{name: "pop-up open", yaml: keysTestMenu, cfg: treeCfg(), steps: seq(runes("workspace"), runes(" "), runes("/"))},
		{name: "flat typed", cfg: Config{Nth: []int{1}}, flat: true, steps: runes("ws")},
	} {
		t.Run(f.name, func(t *testing.T) {
			s, cols := newKeysState(t, f)
			if got := pressKey(s, f, ctrlC, cols).Action; got != "cancel" {
				t.Fatalf("Ctrl+C action = %q, want cancel", got)
			}
			if !s.Cancelled {
				t.Fatal("Ctrl+C didn't set Cancelled")
			}
		})
	}
}

// A rename cut short by Ctrl+C puts the old name back, so a host that saves
// the tree on exit never writes the half-typed one.
func TestCtrlCDuringRenameRestoresTheName(t *testing.T) {
	f := keyFixture{yaml: keysTestMenu, cfg: treeCfg()}
	s, cols := newKeysState(t, f)
	ctx := s.TopCtx()
	idx := -1
	for i, it := range ctx.AllItems {
		if it.Fields[0] == "lights" {
			idx = i
		}
	}
	s.EditMode, s.EditTargetIdx, s.EditOrigName = "rename", idx, "lights"
	ctx.AllItems[idx].Fields[0] = "lig"
	if got := pressKey(s, f, keyPress{key: tcell.KeyCtrlC}, cols).Action; got != "cancel" {
		t.Fatalf("Ctrl+C action = %q, want cancel", got)
	}
	if name := ctx.AllItems[idx].Fields[0]; name != "lights" || s.EditMode != "" {
		t.Fatalf("after Ctrl+C: name %q, edit mode %q; want lights and none", name, s.EditMode)
	}
}
