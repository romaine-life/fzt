package render

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// put writes text one grapheme cluster per call, as a tcell host would.
func put(m *MemScreen, x, y int, clusters ...string) {
	for _, c := range clusters {
		r := []rune(c)
		m.SetContent(x, y, r[0], r[1:], tcell.StyleDefault)
		if c == "a" || c == "b" || c == "|" {
			x++
		} else {
			x += 2
		}
	}
}

func TestWideClustersTakeTwoColumns(t *testing.T) {
	m := NewMemScreen(12, 1)
	put(m, 0, 0, "a", "👍🏽", "👨‍👩‍👧", "😀", "b", "|")
	if got, want := m.Snapshot(), "a👍🏽👨‍👩‍👧😀b|"; got != want {
		t.Fatalf("snapshot %q, want %q", got, want)
	}
	if got := strings.Count(m.ToANSI(), "👍🏽"); got != 1 {
		t.Fatalf("ANSI has the skin-tone cluster %d times: %q", got, m.ToANSI())
	}
	if plain := strings.TrimRight(stripSGR(m.ToANSI()), " "); plain != "a👍🏽👨‍👩‍👧😀b|" {
		t.Fatalf("ANSI text %q", plain)
	}
	primary, combining, _, width := m.Cell(3, 0)
	if string(primary)+string(combining) != "👨‍👩‍👧" || width != 2 {
		t.Fatalf("cell 3 is %q width %d", string(primary)+string(combining), width)
	}
	if _, _, _, width := m.Cell(4, 0); width != 0 {
		t.Fatalf("the family's right half has width %d", width)
	}
	if _, _, _, width := m.Cell(0, 0); width != 1 {
		t.Fatalf("a narrow cell has width %d", width)
	}
}

func TestWritingOverHalfAWideClusterBlanksTheOtherHalf(t *testing.T) {
	m := NewMemScreen(6, 2)
	put(m, 0, 0, "😀", "😀")
	put(m, 1, 0, "b") // the first emoji's right half
	put(m, 2, 0, "a") // the second emoji's left half
	if got := strings.Split(m.Snapshot(), "\n")[0]; got != " ba" {
		t.Fatalf("row 0 %q", got)
	}
	put(m, 5, 1, "😀") // no room for its right half
	if got := strings.Split(m.Snapshot(), "\n")[1]; got != "" {
		t.Fatalf("an emoji at the right edge left %q", got)
	}
}

func stripSGR(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
