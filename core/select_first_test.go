package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func flatItems() []Item {
	return ParseLines([]string{"Repo\tabout", "D:\\repos\tthe folder", "alpha\ta repo"}, "\t", false, false)
}

func TestSelectFirstEnterAtOncePicksTheFirstRow(t *testing.T) {
	cfg := Config{HeaderLines: 1, Delimiter: "\t", SelectFirst: true}
	s, searchCols := NewState(flatItems(), cfg)
	result := HandleKeyEventResult(s, tcell.KeyEnter, 0, false, cfg, searchCols)
	if !result.HasItem || result.Item.Fields[0] != "D:\\repos" {
		t.Fatalf("Enter at once picked %+v (action %q), want the D:\\repos row", result.Item.Fields, result.Action)
	}
}

func TestWithoutSelectFirstEnterAtOncePicksNothing(t *testing.T) {
	cfg := Config{HeaderLines: 1, Delimiter: "\t"}
	s, searchCols := NewState(flatItems(), cfg)
	if result := HandleKeyEventResult(s, tcell.KeyEnter, 0, false, cfg, searchCols); result.Action != "" || result.HasItem {
		t.Fatalf("Enter at once gave %q %+v, want nothing", result.Action, result.Item.Fields)
	}
}
