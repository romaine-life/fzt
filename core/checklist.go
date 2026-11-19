package core

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// A checklist is a folder (Item.Checklist) whose rows are checkboxes, inside
// an ordinary menu. Enter or a click on one of its rows ticks or unticks it
// instead of picking it. The rest of the menu works as it always does, and a
// pick there carries every ticked row, so the rows around a checklist are
// how the person answers ("unsubscribe from these", "not now").

// IsCheckRow reports whether item is one of a checklist's rows in ctx.
func IsCheckRow(ctx *TreeContext, item Item) bool {
	p := item.ParentIdx
	return !item.HasChildren && p >= 0 && p < len(ctx.AllItems) && ctx.AllItems[p].Checklist
}

// HasChecklist reports whether the primary menu holds a checklist.
func HasChecklist(s *State) bool {
	for _, it := range s.Contexts[0].AllItems {
		if it.Checklist && !it.Dropped {
			return true
		}
	}
	return false
}

// ToggleTick ticks or unticks the top context's row idx (an AllItems index),
// when it is a checklist's row.
func ToggleTick(s *State, idx int) {
	ctx := s.TopCtx()
	if idx < 0 || idx >= len(ctx.AllItems) || !IsCheckRow(ctx, ctx.AllItems[idx]) {
		return
	}
	ctx.AllItems[idx].Ticked = !ctx.AllItems[idx].Ticked
}

// Ticked returns the primary menu's ticked rows in list order.
func Ticked(s *State) []Item {
	ctx := &s.Contexts[0]
	var out []Item
	for _, it := range ctx.AllItems {
		if it.Ticked && !it.Dropped && IsCheckRow(ctx, it) {
			out = append(out, it)
		}
	}
	return out
}

// tickKey is Enter (or Shift+Enter, which elsewhere picks the cursor's row
// whatever it is) on a checklist's row: it ticks or unticks the row and moves
// the cursor on, to the next match while searching (the search is kept), else
// down a row, so a run of rows can be ticked one Enter after another. claimed
// is false for any other key or row, which the menu handles as usual.
func tickKey(s *State, key tcell.Key) (claimed bool) {
	if key != tcell.KeyEnter {
		return false
	}
	ctx := s.TopCtx()
	visible := TreeVisibleItems(s)
	if ctx.TreeCursor < 0 || ctx.TreeCursor >= len(visible) || !IsCheckRow(ctx, visible[ctx.TreeCursor].Item) {
		return false
	}
	cur := visible[ctx.TreeCursor]
	ToggleTick(s, cur.ItemIdx)
	if !stepToNextMatch(s, cur.ItemIdx) && ctx.TreeCursor+1 < len(visible) {
		ctx.TreeCursor++
	}
	return true
}

// tickEffect mirrors tickKey for the `/` pop-up.
func tickEffect(s *State, p keyPress, cur *TreeRow) (verb, desc string, ok bool) {
	if p.key != tcell.KeyEnter || cur == nil || !IsCheckRow(s.TopCtx(), cur.Item) {
		return "", "", false
	}
	if cur.Item.Ticked {
		return "untick", "Untick " + itemName(cur.Item), true
	}
	return "tick", "Tick " + itemName(cur.Item), true
}

// stepToNextMatch moves the cursor from the match at AllItems index from to
// the next row in the search's ranking, opening the folders it is in. It
// reports false when no search ranks from, so the cursor moves down a row
// instead; at the last match the cursor stays.
func stepToNextMatch(s *State, from int) bool {
	ctx := s.TopCtx()
	if !ctx.SearchActive || len(ctx.Filtered) == 0 {
		return false
	}
	at := -1
	for i, it := range ctx.Filtered {
		if FindInAll(ctx.AllItems, it) == from {
			at = i
			break
		}
	}
	if at < 0 {
		return false
	}
	for _, it := range ctx.Filtered[at+1:] {
		if it.HasChildren {
			continue
		}
		idx := FindInAll(ctx.AllItems, it)
		if idx < 0 {
			continue
		}
		if ctx.QueryExpanded == nil {
			ctx.QueryExpanded = map[int]bool{}
		}
		for p := ctx.AllItems[idx].ParentIdx; p >= 0; p = ctx.AllItems[p].ParentIdx {
			ctx.QueryExpanded[p] = true
		}
		for vi, row := range TreeVisibleItems(s) {
			if row.ItemIdx == idx {
				ctx.TreeCursor = vi
				return true
			}
		}
	}
	return true
}

// withTicks adds a menu's ticked rows to a pick: Items holds them, and the
// pick's text gains each one's output on a line of its own after the picked
// row's. A menu with no checklist is left as it was.
func withTicks(s *State, cfg Config, res ActionResult) ActionResult {
	if !strings.HasPrefix(res.Action, "select:") || !HasChecklist(s) {
		return res
	}
	res.Items = Ticked(s)
	for _, it := range res.Items {
		res.Action += "\n" + FormatOutput(it, cfg)
	}
	return res
}
