package core

import (
	"slices"
	"strings"
	"unicode"
)

// TagTerm is one query term in a tag search and the tag it resolved to.
// Tag is empty when no tag in the search pool matches the term.
type TagTerm struct {
	Term string `json:"term"`
	Tag  string `json:"tag"`
}

// TagSearchActive reports whether the context's current results come from a
// tag search rather than the tiered scorer.
func TagSearchActive(ctx *TreeContext) bool {
	return len(ctx.TagTerms) > 0 && len(ctx.Query) > 0
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

// resolveTag returns the tag in vocab that best matches term: the highest
// FuzzyMatch score, then the shortest tag (so an exact tag beats a longer one
// it prefixes), then alphabetical order. Empty when nothing matches.
func resolveTag(term string, vocab []string) string {
	best, bestScore := "", 0
	for _, tag := range vocab {
		m := FuzzyMatch(tag, term)
		if m == nil {
			continue
		}
		switch {
		case m.Score > bestScore,
			m.Score == bestScore && len(tag) < len(best),
			m.Score == bestScore && len(tag) == len(best) && tag < best:
			best, bestScore = tag, m.Score
		}
	}
	return best
}

// filterByTags resolves the query's terms to tags left to right, each among
// the tags of the pool items carrying every tag resolved before it, so a
// partly typed term picks a tag that still narrows the results instead of
// one that empties them. An item's name counts as one of its tags. It keeps
// those items, in tree order; any unresolved term leaves no results. It
// returns false, changing nothing, when no pool item has tags or the query
// starts with ':', so that search runs as usual: an
// untagged menu stays searchable with tag mode on, and the `:` palette, where
// tag mode is turned off, stays reachable by typing.
func filterByTags(ctx *TreeContext, pool []Item, query string) bool {
	if strings.HasPrefix(query, ":") || !anyTagged(pool) {
		return false
	}

	terms := strings.Fields(query)
	resolved := make([]TagTerm, len(terms))
	matching := pool
	score := 0
	unresolved := false
	for i, term := range terms {
		tag := resolveTag(term, tagVocabulary(matching))
		resolved[i] = TagTerm{Term: term, Tag: tag}
		if tag == "" {
			unresolved = true
			continue // later terms still resolve, so each one's state is shown
		}
		score += FuzzyMatch(tag, term).Score
		matching = withTag(matching, tag)
	}
	ctx.TagTerms = resolved
	ctx.Filtered = nil
	if unresolved {
		return true
	}

	for _, item := range matching {
		m := item
		m.Score = TieredScore{Name: score}
		m.MatchIndices = make([][]int, len(item.Fields))
		ctx.Filtered = append(ctx.Filtered, m)
	}
	return true
}

func withTag(items []Item, tag string) []Item {
	var out []Item
	for _, item := range items {
		if hasSearchTag(item, tag) {
			out = append(out, item)
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
