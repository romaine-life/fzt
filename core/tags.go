package core

import (
	"slices"
	"strings"
	"unicode"
)

// TagTerm is one query term in a tag search and the tags it matched: every
// tag that starts with it. Tag is the shortest of them, which Tab completes
// to; both are empty when no tag in the search pool starts with the term.
type TagTerm struct {
	Term string   `json:"term"`
	Tag  string   `json:"tag"`
	Tags []string `json:"tags"`
}

// TagSearchActive reports whether the context's current results come from a
// tag search rather than the tiered scorer.
func TagSearchActive(ctx *TreeContext) bool {
	return len(ctx.TagTerms) > 0 && len(ctx.Query) > 0
}

// TagSearchOn reports whether a query typed now searches by tag: tag mode is
// on, or the current scope is inside a TagDefault folder.
func TagSearchOn(s *State) bool {
	if s.TagMode {
		return true
	}
	ctx := s.TopCtx()
	for _, level := range ctx.Scope {
		if level.ParentIdx >= 0 && level.ParentIdx < len(ctx.AllItems) && ctx.AllItems[level.ParentIdx].TagDefault {
			return true
		}
	}
	return false
}

// searchTags is what a tag search matches an item by: its tags and its own
// name, so typing a row's name finds that row.
func searchTags(item Item) []string {
	if len(item.Fields) == 0 || item.Fields[0] == "" || item.HasTag(item.Fields[0]) {
		return item.Tags
	}
	return append(item.Tags[:len(item.Tags):len(item.Tags)], item.Fields[0])
}

func hasSearchTag(item Item, tag string) bool {
	for _, t := range searchTags(item) {
		if t == tag {
			return true
		}
	}
	return false
}

func anyTagged(pool []Item) bool {
	for _, item := range pool {
		if len(item.Tags) > 0 {
			return true
		}
	}
	return false
}

// tagVocabulary returns the distinct search tags of the pool's items, in
// first-seen order.
func tagVocabulary(pool []Item) []string {
	seen := map[string]bool{}
	var vocab []string
	for _, item := range pool {
		for _, tag := range searchTags(item) {
			if !seen[tag] {
				seen[tag] = true
				vocab = append(vocab, tag)
			}
		}
	}
	return vocab
}

// tagsStartingWith returns the tags in vocab that start with term, ignoring
// case, leaving out those in taken. Typing another letter can only drop tags
// from this list, never add one.
func tagsStartingWith(term string, vocab []string, taken map[string]bool) []string {
	var out []string
	for _, tag := range vocab {
		if !taken[tag] && len(tag) >= len(term) && strings.EqualFold(tag[:len(term)], term) {
			out = append(out, tag)
		}
	}
	return out
}

// shortestTag is the tag Tab completes to: the shortest, then alphabetical.
func shortestTag(tags []string) string {
	best := ""
	for _, tag := range tags {
		if best == "" || len(tag) < len(best) || len(tag) == len(best) && tag < best {
			best = tag
		}
	}
	return best
}

// filterByTags keeps the pool items that every query term matches. A term
// matches an item carrying any tag that starts with it, an item's name
// counting as one of its tags, so a row leaves the results only once what is
// typed rules it out: each added letter or term can only narrow them. Each
// term looks only at the tags of the items the earlier terms left, less the
// tags those terms matched, which are locked in (FZT-80). Items stay in tree
// order; a term no tag starts with leaves no results. It returns false,
// changing nothing, when no pool item has tags or the query starts with ':',
// so that search runs as usual: an untagged menu stays searchable with tag
// mode on, and the `:` palette, where tag mode is turned off, stays reachable
// by typing.
func filterByTags(ctx *TreeContext, pool []Item, query string) bool {
	if strings.HasPrefix(query, ":") || !anyTagged(pool) {
		return false
	}

	terms := strings.Fields(query)
	resolved := make([]TagTerm, len(terms))
	matching := pool
	taken := map[string]bool{}
	unresolved := false
	for i, term := range terms {
		tags := tagsStartingWith(term, tagVocabulary(matching), taken)
		resolved[i] = TagTerm{Term: term, Tag: shortestTag(tags), Tags: tags}
		if len(tags) == 0 {
			unresolved = true
			continue // later terms still resolve, so each one's state is shown
		}
		matching = withAnyTag(matching, tags)
		for _, tag := range tags {
			taken[tag] = true
		}
	}
	ctx.TagTerms = resolved
	ctx.Filtered = nil
	if unresolved {
		return true
	}

	for _, item := range matching {
		m := item
		m.MatchIndices = make([][]int, len(item.Fields))
		ctx.Filtered = append(ctx.Filtered, m)
	}
	return true
}

func withAnyTag(items []Item, tags []string) []Item {
	var out []Item
	for _, item := range items {
		for _, tag := range tags {
			if hasSearchTag(item, tag) {
				out = append(out, item)
				break
			}
		}
	}
	return out
}

// HasTag reports whether the item carries tag.
func (item Item) HasTag(tag string) bool {
	for _, t := range item.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// tagResultRows lists a tag search's results as a flat list: each row keeps
// its item index but drops its tree depth, so nothing is indented under a
// parent that isn't shown.
func tagResultRows(ctx *TreeContext) []TreeRow {
	rows := make([]TreeRow, 0, len(ctx.Filtered))
	for _, item := range ctx.Filtered {
		idx := FindInAll(ctx.AllItems, item)
		if idx < 0 {
			continue
		}
		row := ctx.AllItems[idx]
		row.Depth = 0
		rows = append(rows, TreeRow{Item: row, ItemIdx: idx})
	}
	return rows
}

// completeTagTerm replaces the query's last term with the tag it resolved
// to and a space for the next term, the tag-search form of Tab completion.
// It reports whether it changed the query.
func completeTagTerm(ctx *TreeContext) bool {
	if !TagSearchActive(ctx) {
		return false
	}
	last := ctx.TagTerms[len(ctx.TagTerms)-1]
	q := string(ctx.Query)
	cut := strings.LastIndex(q, last.Term)
	if last.Tag == "" || cut < 0 || cut+len(last.Term) < len(q) {
		return false // nothing resolved, or the last term is already finished
	}
	ctx.Query = []rune(q[:cut] + last.Tag + " ")
	ctx.Cursor = len(ctx.Query)
	return true
}

// TagGhost is the rest of the tag the query's last term resolved to, when the
// term is a prefix of it: the autocomplete hint a renderer shows after the
// cursor, which Tab accepts.
func TagGhost(ctx *TreeContext) string {
	if !TagSearchActive(ctx) {
		return ""
	}
	q := string(ctx.Query)
	last := ctx.TagTerms[len(ctx.TagTerms)-1]
	if !strings.HasSuffix(q, last.Term) || len(last.Tag) <= len(last.Term) ||
		!strings.EqualFold(last.Tag[:len(last.Term)], last.Term) {
		return ""
	}
	return last.Tag[len(last.Term):]
}

// ParseTags reads tags typed or stored as one string, separated by spaces or
// commas, dropping repeats.
func ParseTags(s string) []string {
	var tags []string
	for _, t := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
		if !slices.Contains(tags, t) {
			tags = append(tags, t)
		}
	}
	return tags
}

// FormatTags is how the item editor shows tags: space separated.
func FormatTags(tags []string) string {
	return strings.Join(tags, " ")
}
