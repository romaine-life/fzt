package core

import "github.com/rivo/uniseg"

// Width is the columns s takes in a terminal: an emoji or a wide CJK
// character takes two, and a cluster such as a skin-toned or joined emoji is
// one character however many runes it has.
func Width(s string) int {
	return uniseg.StringWidth(s)
}
