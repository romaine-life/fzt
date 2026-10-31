package core

import "strings"

// PanelLines is how many rows the result panel holds. A frontend that shows
// the panel reserves exactly this many rows, empty or not, so the prompt and
// tree never move when a result appears or clears.
const PanelLines = 3

// Panel styles, matching TitleStyle's colors.
const (
	PanelRunning = 3 // neutral: a command is running
	PanelOK      = 1 // green: the command exited 0
	PanelFailed  = 2 // red: the command exited non-zero
)

// Panel is the latest command result a long-lived frontend shows above the
// prompt. Only the latest result is kept; setting one replaces the last.
type Panel struct {
	Lines   []string // at most PanelLines lines
	Style   int      // PanelRunning, PanelOK or PanelFailed
	Clipped bool     // the command printed more than PanelLines lines
}

// SetPanel shows a result, keeping the first PanelLines non-empty lines of
// text and noting whether more were cut.
func (s *State) SetPanel(text string, style int) {
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if l = strings.TrimRight(l, " \t\r"); l != "" {
			lines = append(lines, l)
		}
	}
	clipped := len(lines) > PanelLines
	if clipped {
		lines = lines[:PanelLines]
	}
	s.Panel = Panel{Lines: lines, Style: style, Clipped: clipped}
}

// ClearPanel empties the panel.
func (s *State) ClearPanel() { s.Panel = Panel{} }

// Freeze makes every key and click a no-op until Unfreeze: a frontend runs one
// command at a time and the menu waits for it.
func (s *State) Freeze()   { s.Frozen = true }
func (s *State) Unfreeze() { s.Frozen = false }

// ParseQuick reads a quick property typed into the inspect editor.
func ParseQuick(v string) (quick bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "1":
		return true, true
	case "false", "no", "0", "":
		return false, true
	}
	return false, false
}

// FormatQuick is how the inspect editor shows a quick property.
func FormatQuick(q bool) string {
	if q {
		return "true"
	}
	return "false"
}
