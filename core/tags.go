package core

import "strings"

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

// tagVocabulary returns the distinct tags carried by the pool, in first-seen
// order.
func tagVocabulary(pool []Item) []string {
	seen := map[string]bool{}
	var vocab []string
	for _, item := range pool {
		for _, tag := range item.Tags {
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

// filterByTags resolves each query term to one tag and keeps the pool items
// that carry every resolved tag, in tree order. Any unresolved term leaves no
// results. It returns false, changing nothing, when the pool has no tags or
// the query starts with ':', so that search runs as usual: an untagged menu
// stays searchable with tag mode on, and the `:` palette, where tag mode is
// turned off, stays reachable by typing.
func filterByTags(ctx *TreeContext, pool []Item, query string) bool {
	if strings.HasPrefix(query, ":") {
		return false
	}
	vocab := tagVocabulary(pool)
	if len(vocab) == 0 {
		return false
	}

	terms := strings.Fields(query)
	resolved := make([]TagTerm, len(terms))
	score := 0
	unresolved := false
	for i, term := range terms {
		tag := resolveTag(term, vocab)
		resolved[i] = TagTerm{Term: term, Tag: tag}
		if tag == "" {
			unresolved = true
		} else {
			score += FuzzyMatch(tag, term).Score
		}
	}
	ctx.TagTerms = resolved
	ctx.Filtered = nil
	if unresolved {
		return true
	}

	for _, item := range pool {
		if hasAllTags(item, resolved) {
			m := item
			m.Score = TieredScore{Name: score}
			m.MatchIndices = make([][]int, len(item.Fields))
			ctx.Filtered = append(ctx.Filtered, m)
		}
	}
	return true
}

func hasAllTags(item Item, terms []TagTerm) bool {
	for _, t := range terms {
		if !item.HasTag(t.Tag) {
			return false
		}
	}
	return true
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
