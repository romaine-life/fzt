package render

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// Canvas is the drawing target abstraction. Both tcell.Screen and the
// headless MemScreen implement it, so all draw* functions work with either.
type Canvas interface {
	SetContent(x, y int, primary rune, combining []rune, style tcell.Style)
	Size() (int, int)
	ShowCursor(x, y int)
	HideCursor()
}

// MemScreen is a headless in-memory screen for simulation/testing.
//
// A cell holds a whole grapheme cluster, as a tcell cell does: Grid has its
// first rune and Text all of it. A wide cluster (most emoji) takes its cell
// and the next, whose Text is "" and whose Grid is a space.
type MemScreen struct {
	W, H    int
	Grid    [][]rune
	Text    [][]string
	Styles  [][]tcell.Style
	CursorX int
	CursorY int
}

// NewMemScreen creates a blank in-memory screen.
// Dimensions are clamped to prevent catastrophic allocations.
func NewMemScreen(w, h int) *MemScreen {
	if w < 1 {
		w = 1
	} else if w > 500 {
		w = 500
	}
	if h < 1 {
		h = 1
	} else if h > 200 {
		h = 200
	}
	grid := make([][]rune, h)
	text := make([][]string, h)
	styles := make([][]tcell.Style, h)
	for y := 0; y < h; y++ {
		grid[y] = make([]rune, w)
		text[y] = make([]string, w)
		styles[y] = make([]tcell.Style, w)
		for x := 0; x < w; x++ {
			grid[y][x] = ' '
			text[y][x] = " "
		}
	}
	return &MemScreen{W: w, H: h, Grid: grid, Text: text, Styles: styles}
}

// SetContent puts the cluster primary+combining in cell (x, y). One wider
// than a column takes the next cell too, or becomes a space at the right
// edge; writing over either half of a wide cluster blanks the other half.
func (m *MemScreen) SetContent(x, y int, primary rune, combining []rune, style tcell.Style) {
	if x < 0 || x >= m.W || y < 0 || y >= m.H {
		return
	}
	cluster := string(primary) + string(combining)
	wide := uniseg.StringWidth(cluster) > 1
	if wide && x+1 >= m.W {
		primary, cluster, wide = ' ', " ", false
	}
	m.clear(x, y)
	m.Grid[y][x], m.Text[y][x], m.Styles[y][x] = primary, cluster, style
	if wide {
		m.clear(x+1, y)
		m.Grid[y][x+1], m.Text[y][x+1], m.Styles[y][x+1] = ' ', "", style
	}
}

// clear blanks cell (x, y) and whatever part of a wide cluster it shared.
func (m *MemScreen) clear(x, y int) {
	if m.Text[y][x] == "" && x > 0 {
		m.Grid[y][x-1], m.Text[y][x-1] = ' ', " "
	}
	if x+1 < m.W && m.Text[y][x+1] == "" {
		m.Grid[y][x+1], m.Text[y][x+1] = ' ', " "
	}
	m.Grid[y][x], m.Text[y][x] = ' ', " "
}

func (m *MemScreen) Size() (int, int) {
	return m.W, m.H
}

// GetContent returns the rune and style at the given cell.
func (m *MemScreen) GetContent(x, y int) (rune, tcell.Style) {
	if x >= 0 && x < m.W && y >= 0 && y < m.H {
		return m.Grid[y][x], m.Styles[y][x]
	}
	return ' ', tcell.StyleDefault
}

// Cell returns cell (x, y) as tcell.Screen's GetContent does: its cluster's
// first rune, the rest, its style, and the columns it takes. The right half
// of a wide cluster has width 0, so a caller copying cells to a tcell screen
// skips it.
func (m *MemScreen) Cell(x, y int) (primary rune, combining []rune, style tcell.Style, width int) {
	if x < 0 || x >= m.W || y < 0 || y >= m.H {
		return ' ', nil, tcell.StyleDefault, 1
	}
	text := m.Text[y][x]
	if text == "" {
		return ' ', nil, m.Styles[y][x], 0
	}
	runes := []rune(text)
	width = 1
	if x+1 < m.W && m.Text[y][x+1] == "" {
		width = 2
	}
	return runes[0], runes[1:], m.Styles[y][x], width
}

func (m *MemScreen) ShowCursor(x, y int) {
	m.CursorX = x
	m.CursorY = y
}

func (m *MemScreen) HideCursor() {
	m.CursorX = -1
	m.CursorY = -1
}

// Clear resets all cells to spaces.
func (m *MemScreen) Clear() {
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			m.Grid[y][x] = ' '
			m.Text[y][x] = " "
			m.Styles[y][x] = tcell.StyleDefault
		}
	}
}

// Snapshot returns the grid as a string, trimming trailing whitespace per line.
func (m *MemScreen) Snapshot() string {
	var lines []string
	for y := 0; y < m.H; y++ {
		var line string
		for x := 0; x < m.W; x++ {
			line += m.Text[y][x]
		}
		lines = append(lines, trimRight(line))
	}
	return joinLines(lines)
}

// StyledSnapshot returns the grid annotated with style markers.
// [H] = highlighted (green/bold), [S] = selected (blue bg), [*] = both.
// Plain characters have no marker.
func (m *MemScreen) StyledSnapshot() string {
	var lines []string
	for y := 0; y < m.H; y++ {
		var line []rune
		for x := 0; x < m.W; x++ {
			ch := m.Text[y][x]
			if ch == "" {
				continue // the right half of a wide cluster
			}
			style := m.Styles[y][x]
			fg, bg, attrs := style.Decompose()
			isHL := fg == tcell.ColorGreen && attrs&tcell.AttrBold != 0
			isSel := bg == tcell.ColorDarkBlue

			if isHL && isSel {
				line = append(line, '[', '*', ']')
			} else if isHL {
				line = append(line, '[', 'H', ']')
			} else if isSel {
				line = append(line, '[', 'S', ']')
			}
			line = append(line, []rune(ch)...)
		}
		trimmed := trimRight(string(line))
		lines = append(lines, trimmed)
	}
	return joinLines(lines)
}

func trimRight(s string) string {
	runes := []rune(s)
	end := len(runes)
	for end > 0 && runes[end-1] == ' ' {
		end--
	}
	return string(runes[:end])
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
