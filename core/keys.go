package core

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// KeyAction is one thing that can be done at the moment: a row of the `/`
// pop-up. Verb is what the pop-up's search matches, Desc says what it does
// now, and Keys are the keys that do it, drawn as keycaps.
type KeyAction struct {
	Verb  string
	Desc  string
	Keys  []string
	press []keyPress // what picking the row presses; empty when run does it
	run   func(s *State, cfg Config, searchCols []int) ActionResult
}

type keyPress struct {
	key   tcell.Key
	ch    rune
	shift bool
	label string // its keycap; empty when another key's cap stands for it (Space typing)
}

// keyPresses are the keys the pop-up asks about, in the order their caps
// appear within a row.
var keyPresses = []keyPress{
	{key: tcell.KeyEnter, label: "Enter"},
	{key: tcell.KeyEnter, shift: true, label: "Shift+Enter"},
	{key: tcell.KeyTab, label: "Tab"},
	{key: tcell.KeyBacktab, label: "Shift+Tab"},
	{key: tcell.KeyUp, label: "↑"},
	{key: tcell.KeyDown, label: "↓"},
	{key: tcell.KeyLeft, label: "←"},
	{key: tcell.KeyRight, label: "→"},
	{key: tcell.KeyRune, ch: 'k', label: "k"},
	{key: tcell.KeyRune, ch: 'j', label: "j"},
	{key: tcell.KeyRune, ch: 'h', label: "h"},
	{key: tcell.KeyRune, ch: 'l', label: "l"},
	{key: tcell.KeyRune, ch: ' ', label: "Space"},
	{key: tcell.KeyRune, ch: 'a', label: "a-z"},
	{key: tcell.KeyBackspace2, label: "Backspace"},
	{key: tcell.KeyDelete, label: "Delete"},
	{key: tcell.KeyHome, label: "Home"},
	{key: tcell.KeyEnd, label: "End"},
	{key: tcell.KeyRune, ch: '`', label: "`"},
	{key: tcell.KeyEscape, label: "Esc"},
	{key: tcell.KeyBackspace2, shift: true, label: "Shift+Backspace"},
}

// verbOrder ranks the pop-up's rows: acting on the match first, then moving,
// typing, modes and leaving.
var verbOrder = []string{
	"run", "pick", "open folder", "open link", "complete", "complete tag",
	"next match", "previous match", "walk rows", "expand folder", "close folder",
	"go into folder", "go to parent", "type", "search", "type again",
	"delete character", "delete forward", "move text cursor",
	"switch to normal mode", "clear search", "leave folder", "go to top", "close menu",
}

const normalModeDesc = "Stop typing; arrows and h j k l move the cursor"

// KeyActions lists what can be done now, one row per verb with every key
// that does it. flat is true for fzt's flat list (HandleKeyEvent), false for
// the tree (HandleUnifiedKey). The rows come from keyEffect, which mirrors
// the key handlers branch for branch; keys_test checks that a key is listed
// exactly when pressing it changes something.
func KeyActions(s *State, cfg Config, flat bool) []KeyAction {
	var rows []KeyAction
	add := func(verb, desc string, p keyPress) {
		for i := range rows {
			if rows[i].Verb == verb && rows[i].Desc == desc {
				rows[i].press = append(rows[i].press, p)
				if p.label != "" && !hasString(rows[i].Keys, p.label) {
					rows[i].Keys = append(rows[i].Keys, p.label)
				}
				return
			}
		}
		row := KeyAction{Verb: verb, Desc: desc, press: []keyPress{p}}
		if p.label != "" {
			row.Keys = []string{p.label}
		}
		rows = append(rows, row)
	}
	for _, p := range keyPresses {
		var verb, desc string
		var ok bool
		if flat {
			verb, desc, ok = flatEffect(s, cfg, p)
		} else {
			verb, desc, ok = unifiedEffect(s, cfg, p)
		}
		if !ok {
			continue
		}
		typing := verb == "type" || desc == "Start a search by typing"
		if (p.ch == ' ' && typing) || (p.ch != 'a' && p.key == tcell.KeyRune && strings.ContainsRune("hjkl", p.ch) && typing) {
			p.label = "" // typing, which the a-z cap already shows
		}
		add(verb, desc, p)
	}
	if !flat {
		ctx := s.TopCtx()
		if ctx.NavMode && !hasVerb(rows, "search", "Go back to typing") {
			rows = append(rows, KeyAction{Verb: "search", Desc: "Go back to typing", run: resumeTyping})
		}
	}
	sortKeyActions(rows)
	return rows
}

func resumeTyping(s *State, cfg Config, searchCols []int) ActionResult {
	ctx := s.TopCtx()
	ctx.NavMode = false
	ctx.SearchActive = true
	s.SetTitle("", 5)
	return noActionResult()
}

func hasString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func hasVerb(rows []KeyAction, verb, desc string) bool {
	for _, r := range rows {
		if r.Verb == verb && r.Desc == desc {
			return true
		}
	}
	return false
}

func sortKeyActions(rows []KeyAction) {
	rank := func(v string) int {
		for i, o := range verbOrder {
			if o == v {
				return i
			}
		}
		return len(verbOrder)
	}
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rank(rows[j].Verb) < rank(rows[j-1].Verb); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func itemName(it Item) string {
	if len(it.Fields) > 0 {
		return it.Fields[0]
	}
	return ""
}

// selectEffect is what selecting an item does: a leaf runs, a folder (whose
// Enter would open it) is picked as itself.
func selectEffect(it Item) (string, string, bool) {
	if it.HasChildren {
		return "pick", "Pick the " + itemName(it) + " folder itself", true
	}
	return "run", "Run " + itemName(it), true
}

// enterEffect mirrors Enter on a row in the tree handlers.
func enterEffect(s *State, cfg Config, it Item, idx int) (string, string, bool) {
	ctx := s.TopCtx()
	if it.HasChildren && picksFolders(s, cfg) {
		return "pick", "Pick " + itemName(it), true
	}
	if it.HasChildren {
		if ctx.Scope[len(ctx.Scope)-1].ParentIdx == idx {
			if cfg.FoldersOnly {
				return "pick", "Pick " + itemName(it), true
			}
			if it.Action != nil && it.Action.Type == "url" {
				return "open link", "Open " + itemName(it) + "'s link", true
			}
			return "", "", false
		}
		return "open folder", "Open " + itemName(it), true
	}
	return "run", "Run " + itemName(it), true
}

func folderName(ctx *TreeContext) string {
	p := ctx.Scope[len(ctx.Scope)-1].ParentIdx
	if p < 0 || p >= len(ctx.AllItems) {
		return ""
	}
	return itemName(ctx.AllItems[p])
}

func leaveEffect(ctx *TreeContext) (string, string, bool) {
	return "leave folder", "Leave " + folderName(ctx), true
}

func atHome(s *State) bool {
	ctx := s.TopCtx()
	return len(s.Contexts) == 1 && len(ctx.Scope) <= 1 && len(ctx.Query) == 0 &&
		ctx.TreeCursor < 0 && !ctx.SearchActive && !ctx.NavMode && len(ctx.TreeExpanded) == 0
}

// walkChanges reports whether moving the tree cursor by delta (wrapping, the
// way Up and Down do) changes the cursor, the mode or, with sync, the query.
func walkChanges(ctx *TreeContext, visible []TreeRow, delta int, sync bool) bool {
	if len(visible) == 0 {
		return false
	}
	next := ctx.TreeCursor
	switch {
	case delta > 0 && next < 0:
		next = 0
	case delta > 0:
		next = (next + 1) % len(visible)
	case next <= 0:
		next = len(visible) - 1
	default:
		next--
	}
	if !ctx.NavMode || next != ctx.TreeCursor {
		return true
	}
	return sync && !TagSearchActive(ctx) && string(ctx.Query) != itemName(visible[next].Item)
}

// unifiedEffect says what pressing p does in the tree (HandleUnifiedKey),
// branch for branch with the handlers in input.go.
func unifiedEffect(s *State, cfg Config, p keyPress) (string, string, bool) {
	ctx := s.TopCtx()
	visible := TreeVisibleItems(s)
	var cur *TreeRow
	if ctx.TreeCursor >= 0 && ctx.TreeCursor < len(visible) {
		cur = &visible[ctx.TreeCursor]
	}
	inFolder := len(ctx.Scope) > 1
	backspace := p.key == tcell.KeyBackspace2

	if backspace && p.shift {
		if atHome(s) {
			return "", "", false
		}
		return "go to top", "Leave every folder and clear the search", true
	}
	if p.shift && p.key == tcell.KeyEnter {
		if cur != nil {
			return selectEffect(cur.Item)
		}
		if len(ctx.Filtered) > 0 {
			return selectEffect(ctx.Filtered[0])
		}
		return "", "", false
	}
	if ctx.NavMode && backspace {
		if cur != nil && len(cur.Item.Fields) > 0 {
			name := []rune(cur.Item.Fields[0])
			if len(name) > 1 {
				return "type again", "Go back to typing, from " + string(name[:len(name)-1]), true
			}
		}
		return "search", "Go back to typing", true
	}
	if !ctx.SearchActive {
		if p.key == tcell.KeyRune {
			switch {
			case p.ch == '`':
				if !ctx.NavMode || (ctx.TreeCursor < 0 && len(visible) > 0) {
					return "switch to normal mode", normalModeDesc, true
				}
				return "", "", false
			case p.ch == ' ' && cur != nil && cur.Item.HasChildren:
				return "open folder", "Open " + itemName(cur.Item), true
			case ctx.NavMode:
				if navKey, _, ok := normalModeNavBinding(p.ch); ok {
					return treeEffect(s, cfg, navKey, visible, cur)
				}
				return "", "", false
			case p.ch == ' ':
				return "search", "Start a search by typing", true // shown by the a-z cap
			}
			return "search", "Start a search by typing", true
		}
		return treeEffect(s, cfg, p.key, visible, cur)
	}
	return searchEffect(s, cfg, p, visible, cur, inFolder)
}

// treeEffect mirrors HandleTreeKeyResult: no search active.
func treeEffect(s *State, cfg Config, key tcell.Key, visible []TreeRow, cur *TreeRow) (string, string, bool) {
	ctx := s.TopCtx()
	switch key {
	case tcell.KeyUp, tcell.KeyBacktab:
		if walkChanges(ctx, visible, -1, false) {
			return "walk rows", "Move the cursor through the list", true
		}
	case tcell.KeyDown, tcell.KeyTab:
		if walkChanges(ctx, visible, 1, false) {
			return "walk rows", "Move the cursor through the list", true
		}
	case tcell.KeyEnter:
		if cur != nil {
			return enterEffect(s, cfg, cur.Item, cur.ItemIdx)
		}
	case tcell.KeyRight:
		if cur != nil && cur.Item.HasChildren {
			return "open folder", "Open " + itemName(cur.Item), true
		}
		if !ctx.NavMode {
			return "switch to normal mode", normalModeDesc, true
		}
	case tcell.KeyLeft:
		if s.ScopedView {
			if len(ctx.Scope) > 1 {
				return leaveEffect(ctx)
			}
			return "", "", false
		}
		if v, d, ok := parentEffect(ctx, visible, cur); ok {
			return v, d, ok
		}
		if !ctx.NavMode {
			return "switch to normal mode", normalModeDesc, true
		}
	case tcell.KeyBackspace2:
		if len(ctx.Scope) > 1 {
			return leaveEffect(ctx)
		}
	case tcell.KeyEscape:
		if len(ctx.Scope) > 1 {
			return leaveEffect(ctx)
		}
		return "close menu", "Close the menu", true
	}
	return "", "", false
}

// parentEffect is Left on the cursor's row: close its open folder, else move
// to its parent.
func parentEffect(ctx *TreeContext, visible []TreeRow, cur *TreeRow) (string, string, bool) {
	if cur == nil {
		return "", "", false
	}
	if cur.Item.HasChildren && ctx.TreeExpanded[cur.ItemIdx] {
		return "close folder", "Close " + itemName(cur.Item), true
	}
	if p := cur.Item.ParentIdx; p >= 0 {
		for _, vr := range visible {
			if vr.ItemIdx == p {
				return "go to parent", "Move the cursor up to " + itemName(ctx.AllItems[p]), true
			}
		}
	}
	return "", "", false
}

// searchEffect mirrors HandleSearchKeyResult: a search is active.
func searchEffect(s *State, cfg Config, p keyPress, visible []TreeRow, cur *TreeRow, inFolder bool) (string, string, bool) {
	ctx := s.TopCtx()
	switch p.key {
	case tcell.KeyEscape:
		switch {
		case len(ctx.Query) > 0 && !ctx.NavMode:
			return "switch to normal mode", normalModeDesc, true
		case len(ctx.Query) > 0:
			return "clear search", "Clear the search", true
		case inFolder:
			return leaveEffect(ctx)
		}
		return "close menu", "Close the menu", true
	case tcell.KeyUp:
		if walkChanges(ctx, visible, -1, true) {
			return "walk rows", "Move the cursor through the list; the search follows it", true
		}
	case tcell.KeyDown:
		if walkChanges(ctx, visible, 1, true) {
			return "walk rows", "Move the cursor through the list; the search follows it", true
		}
	case tcell.KeyTab:
		if TagSearchActive(ctx) {
			probe := *ctx
			if completeTagTerm(&probe) {
				return "complete tag", "Complete the tag", true
			}
			return "", "", false
		}
		if target, step, ok := tabTarget(s, cfg, 1); ok {
			if target.HasChildren && picksFolders(s, cfg) {
				return "open folder", "Open " + itemName(target), true
			}
			if step {
				return "next match", "Step to " + itemName(target), true
			}
			return "complete", "Complete to " + itemName(target), true
		}
	case tcell.KeyBacktab:
		if TagSearchActive(ctx) {
			return "", "", false
		}
		if target, _, ok := tabTarget(s, cfg, -1); ok {
			return "previous match", "Step back to " + itemName(target), true
		}
	case tcell.KeyEnter:
		if cur != nil {
			return enterEffect(s, cfg, cur.Item, cur.ItemIdx)
		}
		if s.ScopedView && len(ctx.Query) == 0 {
			scope := ctx.Scope[len(ctx.Scope)-1].ParentIdx
			if cfg.FoldersOnly && scope >= 0 {
				return "pick", "Pick " + itemName(ctx.AllItems[scope]), true
			}
			return "", "", false
		}
		if len(ctx.Filtered) > 0 {
			top := ctx.Filtered[0]
			return enterEffect(s, cfg, top, FindInAll(ctx.AllItems, top))
		}
	case tcell.KeyBackspace2:
		switch {
		case len(ctx.Query) == 0 && inFolder:
			return leaveEffect(ctx)
		case len(ctx.Query) > 0:
			return "delete character", "Delete the last character", true
		}
	case tcell.KeyLeft:
		if s.ScopedView {
			if inFolder {
				return leaveEffect(ctx)
			}
			return "", "", false
		}
		if v, d, ok := parentEffect(ctx, visible, cur); ok {
			return v, d, ok
		}
		if cur != nil && ctx.NavMode {
			return "search", "Go back to typing", true
		}
	case tcell.KeyRight:
		if cur != nil && cur.Item.HasChildren {
			switch {
			case s.ScopedView:
				return "open folder", "Open " + itemName(cur.Item), true
			case !ctx.TreeExpanded[cur.ItemIdx]:
				return "expand folder", "Show what's in " + itemName(cur.Item), true
			case ctx.TreeCursor+1 < len(visible):
				return "go into folder", "Move the cursor into " + itemName(cur.Item), true
			}
		}
		if !ctx.NavMode {
			return "switch to normal mode", normalModeDesc, true
		}
	case tcell.KeyRune:
		if ctx.NavMode {
			if navKey, _, ok := normalModeNavBinding(p.ch); ok {
				return searchEffect(s, cfg, keyPress{key: navKey}, visible, cur, inFolder)
			}
			return "", "", false
		}
		if p.ch == '`' {
			return "switch to normal mode", normalModeDesc, true
		}
		if p.ch == ' ' && !TagSearchActive(ctx) {
			if cur != nil && cur.Item.HasChildren {
				return "open folder", "Open " + itemName(cur.Item), true
			}
			if cur == nil && len(ctx.Filtered) > 0 && ctx.Filtered[0].HasChildren {
				return "open folder", "Open " + itemName(ctx.Filtered[0]), true
			}
		}
		return "type", "Keep typing to narrow the search", true
	}
	return "", "", false
}

// flatEffect mirrors HandleKeyEventResult: fzt's flat list.
func flatEffect(s *State, cfg Config, p keyPress) (string, string, bool) {
	ctx := s.TopCtx()
	var cur *Item
	if ctx.Index >= 0 && ctx.Index < len(ctx.Filtered) {
		cur = &ctx.Filtered[ctx.Index]
	}
	inFolder := cfg.Tiered && len(ctx.Scope) > 1
	if p.shift && p.key == tcell.KeyEnter {
		if cur != nil {
			return selectEffect(*cur)
		}
		return "", "", false
	}
	if p.shift {
		return "", "", false // Shift+Backspace is the tree's
	}
	switch p.key {
	case tcell.KeyEscape:
		switch {
		case len(ctx.Query) > 0:
			return "clear search", "Clear the search", true
		case inFolder:
			return leaveEffect(ctx)
		}
		return "close menu", "Close the list", true
	case tcell.KeyEnter:
		if cur != nil {
			if cur.HasChildren && cfg.Tiered {
				return "open folder", "Open " + itemName(*cur), true
			}
			return "run", "Run " + itemName(*cur), true
		}
	case tcell.KeyBackspace2:
		if ctx.Cursor > 0 {
			return "delete character", "Delete the character before the text cursor", true
		}
	case tcell.KeyDelete:
		if ctx.Cursor < len(ctx.Query) {
			return "delete forward", "Delete the character after the text cursor", true
		}
	case tcell.KeyLeft:
		switch {
		case cfg.Tiered && len(ctx.Query) == 0 && len(ctx.Scope) > 1:
			return leaveEffect(ctx)
		case ctx.Index >= 0:
			return "search", "Go back to the search box", true
		case ctx.Cursor > 0:
			return "move text cursor", "Move the text cursor", true
		}
	case tcell.KeyRight:
		switch {
		case ctx.Index >= 0 && cfg.Tiered && len(ctx.Query) == 0 && cur != nil && cur.HasChildren:
			return "open folder", "Open " + itemName(*cur), true
		case ctx.Index == -1 && ctx.Cursor < len(ctx.Query):
			return "move text cursor", "Move the text cursor", true
		}
	case tcell.KeyHome:
		if ctx.Cursor > 0 {
			return "move text cursor", "Move the text cursor", true
		}
	case tcell.KeyEnd:
		if ctx.Cursor < len(ctx.Query) {
			return "move text cursor", "Move the text cursor", true
		}
	case tcell.KeyTab:
		if len(ctx.Filtered) > 0 {
			if ctx.Index+1 < len(ctx.Filtered) {
				return "next match", "Step to " + itemName(ctx.Filtered[ctx.Index+1]), true
			}
			return "search", "Go back to the search box", true
		}
	case tcell.KeyBacktab:
		if len(ctx.Filtered) > 0 {
			switch {
			case ctx.Index == -1:
				return "previous match", "Step back to " + itemName(ctx.Filtered[len(ctx.Filtered)-1]), true
			case ctx.Index > 0:
				return "previous match", "Step back to " + itemName(ctx.Filtered[ctx.Index-1]), true
			}
			return "search", "Go back to the search box", true
		}
	case tcell.KeyUp:
		switch {
		case ctx.Index > 0:
			return "previous match", "Step back to " + itemName(ctx.Filtered[ctx.Index-1]), true
		case ctx.Index == 0:
			return "search", "Go back to the search box", true
		}
	case tcell.KeyDown:
		if ctx.Index < len(ctx.Filtered)-1 {
			return "next match", "Step to " + itemName(ctx.Filtered[ctx.Index+1]), true
		}
	case tcell.KeyRune:
		return "type", "Type to narrow the list", true
	}
	return "", "", false
}

// tabTarget is the match Tab (dir 1) or Shift+Tab (dir -1) goes to, and
// whether it steps from a locked-in match rather than completing the query.
// The ranking is the one from the query as typed: kept from the first Tab
// until a key other than Tab or Shift+Tab is pressed.
func tabTarget(s *State, cfg Config, dir int) (Item, bool, bool) {
	ctx := s.TopCtx()
	ranking := ctx.tabRanking
	at := ctx.tabAt
	if ranking == nil {
		ranking = ctx.Filtered
		at = -1
		if len(ranking) > 0 && strings.EqualFold(string(ctx.Query), itemName(ranking[0])) {
			at = 0
		}
	}
	if len(ranking) == 0 {
		return Item{}, false, false
	}
	if at < 0 {
		if dir > 0 {
			return ranking[0], false, true
		}
		return ranking[len(ranking)-1], false, true
	}
	if len(ranking) < 2 {
		return Item{}, false, false
	}
	return ranking[(at+dir+len(ranking))%len(ranking)], true, true
}

// tabStep completes the query to a match, or steps through the ranking from
// the query as typed: Tab forward, Shift+Tab back. It never opens a folder,
// except in a folder picker, where Tab goes in (FZT-51).
func tabStep(s *State, cfg Config, searchCols []int, dir int) {
	ctx := s.TopCtx()
	if ctx.tabRanking == nil {
		ctx.tabRanking = append([]Item(nil), ctx.Filtered...)
		ctx.tabAt = -1
		if len(ctx.tabRanking) > 0 && strings.EqualFold(string(ctx.Query), itemName(ctx.tabRanking[0])) {
			ctx.tabAt = 0
		}
	}
	n := len(ctx.tabRanking)
	if n == 0 {
		return
	}
	next := ctx.tabAt + dir
	switch {
	case ctx.tabAt < 0 && dir > 0:
		next = 0
	case ctx.tabAt < 0:
		next = n - 1
	case n < 2:
		return
	}
	next = (next + n) % n
	target := ctx.tabRanking[next]
	if dir > 0 && target.HasChildren && picksFolders(s, cfg) {
		ctx.tabRanking = nil
		if idx := FindInAll(ctx.AllItems, target); idx >= 0 {
			PushScope(s, idx, cfg, searchCols)
		}
		return
	}
	ranking := ctx.tabRanking
	ctx.Query = []rune(itemName(target))
	ctx.Cursor = len(ctx.Query)
	FilterItems(s, cfg, searchCols)
	// The stepped-to match is the top match, whatever else its name matches.
	for i, it := range ctx.Filtered {
		if it.Index == target.Index {
			copy(ctx.Filtered[1:i+1], ctx.Filtered[:i])
			ctx.Filtered[0] = it
			break
		}
	}
	UpdateQueryExpansion(s)
	SyncTreeCursorToTopMatch(s)
	ctx.tabRanking = ranking
	ctx.tabAt = next
}

// KeyPopupOpen reports whether the `/` pop-up is on top.
func KeyPopupOpen(s *State) bool {
	return s.TopCtx().Kind == ContextKeys
}

// OpenKeyPopup puts the `/` pop-up on top: its own fzt over the one you were
// in, listing KeyActions for that moment. The context underneath keeps its
// query and cursor and gets them back when the pop-up closes.
func OpenKeyPopup(s *State, cfg Config, flat bool) {
	actions := KeyActions(s, cfg, flat)
	items := make([]Item, len(actions))
	nameW := 0
	for i, a := range actions {
		items[i] = Item{
			Fields:    []string{a.Verb, a.Desc},
			Keys:      a.Keys,
			ParentIdx: -1,
			Index:     i,
			Action:    &ItemAction{Type: "key", Target: strconv.Itoa(i)},
		}
		if w := len([]rune(a.Verb)); w > nameW {
			nameW = w
		}
	}
	title := keyPopupTitle(s, flat)
	s.PushContext(TreeContext{
		AllItems:     items,
		Items:        RootItemsOf(items),
		NameColWidth: nameW,
		ColGap:       2,
		Index:        -1,
		TreeCursor:   0, // Enter runs the first row; typing searches

		TreeExpanded:  map[int]bool{},
		QueryExpanded: map[int]bool{},
		Scope:         []ScopeLevel{{ParentIdx: -1}},
		Kind:          ContextKeys,
		PromptIcon:    '/',
		keyActions:    actions,
		keyFlat:       flat,
		keyFor:        title,
	})
}

// KeyPopupTitle names what the pop-up acts on: the match or row the context
// underneath has, else its folder.
func KeyPopupTitle(s *State) string {
	return s.TopCtx().keyFor
}

func keyPopupTitle(s *State, flat bool) string {
	ctx := s.TopCtx()
	if flat {
		if ctx.Index >= 0 && ctx.Index < len(ctx.Filtered) {
			return itemName(ctx.Filtered[ctx.Index])
		}
		return ""
	}
	visible := TreeVisibleItems(s)
	if ctx.TreeCursor >= 0 && ctx.TreeCursor < len(visible) {
		return itemName(visible[ctx.TreeCursor].Item)
	}
	if ctx.SearchActive && len(ctx.Filtered) > 0 {
		return itemName(ctx.Filtered[0])
	}
	if len(ctx.Scope) > 1 {
		return folderName(ctx)
	}
	return ""
}

// handleKeyPopup takes a key while the pop-up is on top. It is an fzt like any
// other (type to search, arrows, Enter); `/` closes it, and picking a row
// closes it and does that row's action in the fzt underneath.
func handleKeyPopup(s *State, key tcell.Key, ch rune, shift bool, cfg Config, searchCols []int) ActionResult {
	if key == tcell.KeyRune && ch == '/' {
		s.PopContext()
		return noActionResult()
	}
	return finishKeyPopup(s, cfg, searchCols, unifiedKey(s, key, ch, shift, cfg, searchCols))
}

// finishKeyPopup runs the action of a pop-up row a key or click picked.
func finishKeyPopup(s *State, cfg Config, searchCols []int, res ActionResult) ActionResult {
	if !res.HasItem || res.Item.Action == nil || res.Item.Action.Type != "key" {
		return res
	}
	ctx := s.TopCtx()
	i, err := strconv.Atoi(res.Item.Action.Target)
	if err != nil || i < 0 || i >= len(ctx.keyActions) {
		return noActionResult()
	}
	a, flat := ctx.keyActions[i], ctx.keyFlat
	s.PopContext()
	return RunKeyAction(s, a, flat, cfg, searchCols)
}

// ClickKeyPopupRow picks the pop-up's vi-th visible row, as a click on it
// does: a renderer that draws the pop-up in a box of its own maps the click
// to the row.
func ClickKeyPopupRow(s *State, vi int, cfg Config) ActionResult {
	if !KeyPopupOpen(s) {
		return noActionResult()
	}
	visible := TreeVisibleItems(s)
	if vi < 0 || vi >= len(visible) {
		return noActionResult()
	}
	searchCols := cfg.SearchCols
	if len(searchCols) == 0 {
		searchCols = cfg.Nth
	}
	return finishKeyPopup(s, cfg, searchCols, selectActionResult(visible[vi].Item, cfg))
}

// KeyPopupBox is where the pop-up is drawn in a frame w wide and h tall,
// from the frame's top-left: its own bordered fzt, below the row it acts on,
// or above it when there's no room below, so that row stays in sight. Over
// a flat list it is centered. Renderers draw it there and clicks map
// through it, so the two can't disagree.
func KeyPopupBox(s *State, cfg Config, w, h int) (x, y, bw, bh int) {
	if !KeyPopupOpen(s) || len(s.Contexts) < 2 {
		return 0, 0, 0, 0
	}
	popup := s.TopCtx()
	bw = min(w-6, 100)
	if bw < 30 {
		bw = w
	}
	x = (w - bw) / 2
	bh = min(len(popup.AllItems)+5, h)
	if popup.keyFlat {
		return x, (h - bh) / 2, bw, bh
	}
	under := &s.Contexts[len(s.Contexts)-2]
	first := firstItemRow(under, cfg)
	bottom := h
	if cfg.Border {
		bottom = h - 1
	}
	anchor := first - 1 // the prompt bar's bottom edge
	if under.TreeCursor >= 0 {
		anchor = first + under.TreeCursor - under.TreeOffset
	}
	switch {
	case anchor+1+bh <= bottom:
		y = anchor + 1
	case anchor-bh >= first:
		y = anchor - bh
	default:
		y = max(bottom-bh, 0)
	}
	return x, y, bw, bh
}

// clickKeyPopup is a click while the pop-up is drawn in its box: a click on
// one of its rows picks that row, a click outside closes it.
func clickKeyPopup(s *State, row int, cfg Config, h int) ActionResult {
	_, y, _, bh := KeyPopupBox(s, cfg, 0, h)
	if row < y || row >= y+bh {
		s.PopContext()
		return noActionResult()
	}
	vi := row - (y + 4) + s.TopCtx().TreeOffset // its border and prompt bar
	if row < y+4 {
		return noActionResult()
	}
	return ClickKeyPopupRow(s, vi, cfg)
}

// RunKeyAction does a KeyAction in the fzt on top: it presses the row's first
// key, or runs what a row with no key of its own does.
func RunKeyAction(s *State, a KeyAction, flat bool, cfg Config, searchCols []int) ActionResult {
	if a.run != nil {
		return a.run(s, cfg, searchCols)
	}
	if len(a.press) == 0 {
		return noActionResult()
	}
	p := a.press[0]
	if p.key == tcell.KeyBackspace2 && p.shift {
		GoHome(s, cfg, searchCols)
		return noActionResult()
	}
	if flat {
		return HandleKeyEventResult(s, p.key, p.ch, p.shift, cfg, searchCols)
	}
	return HandleUnifiedKeyResult(s, p.key, p.ch, p.shift, cfg, searchCols)
}

// GoHome is Shift+Backspace: every pushed context closed, every folder left,
// the search cleared.
func GoHome(s *State, cfg Config, searchCols []int) {
	for len(s.Contexts) > 1 {
		s.PopContext()
	}
	ctx := s.TopCtx()
	ctx.Scope = []ScopeLevel{{ParentIdx: -1}}
	if cfg.Tiered {
		ctx.Items = RootItemsOf(ctx.AllItems)
	}
	ctx.Query = nil
	ctx.Cursor = 0
	ctx.TreeCursor = -1
	ctx.TreeOffset = 0
	ctx.SearchActive = false
	ctx.NavMode = false
	ctx.Filtered = nil
	ctx.tabRanking = nil
	ctx.QueryExpanded = make(map[int]bool)
	ctx.TreeExpanded = make(map[int]bool)
	if s.EditMode == "" {
		s.SetTitle("⌂", 1)
	}
}
