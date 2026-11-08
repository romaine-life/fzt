package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// startState is a scoped folder picker over the real drive roots, as
// fzt-picker builds it.
func startState(t *testing.T) (*State, []int, Config) {
	t.Helper()
	cfg := Config{Tiered: true, ScopedView: true, FoldersOnly: true, Provider: NewDirProvider()}
	s, searchCols := NewState(ListDriveRoots(), cfg)
	s.Provider = cfg.Provider
	ctx := s.TopCtx()
	ctx.TreeExpanded = make(map[int]bool)
	ctx.QueryExpanded = make(map[int]bool)
	ctx.TreeCursor = -1
	return s, searchCols, cfg
}

// tempTree makes dir/a/b and returns dir by its long name (a runner's TEMP
// can be an 8.3 short name, which no folder listing shows).
func tempTree(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func scopeFolder(s *State) string {
	ctx := s.TopCtx()
	return ItemFullPath(ctx, ctx.Scope[len(ctx.Scope)-1].ParentIdx)
}

func TestExpandToPathReachesTheFolder(t *testing.T) {
	dir := tempTree(t)
	s, searchCols, cfg := startState(t)
	target := filepath.Join(dir, "a", "b")

	if !ExpandToPath(s, target, cfg, searchCols) {
		t.Fatal("reported unreached for a folder that exists")
	}
	if got := scopeFolder(s); got != target {
		t.Errorf("opened in %q, want %q", got, target)
	}
	if s.TitleOverride != "" {
		t.Errorf("title says %q after reaching the folder", s.TitleOverride)
	}
}

func TestExpandToPathSaysWhenAFolderIsMissing(t *testing.T) {
	dir := tempTree(t)
	s, searchCols, cfg := startState(t)
	target := filepath.Join(dir, "a", "gone", "deeper")

	if ExpandToPath(s, target, cfg, searchCols) {
		t.Fatal("reported reached for a folder that doesn't exist")
	}
	if got, want := scopeFolder(s), filepath.Join(dir, "a"); got != want {
		t.Errorf("opened in %q, want the last folder found, %q", got, want)
	}
	if want := "no folder " + target; s.TitleOverride != want || s.TitleStyle != 2 {
		t.Errorf("title = %q style %d, want %q in red", s.TitleOverride, s.TitleStyle, want)
	}

	HandleUnifiedKeyResult(s, tcell.KeyDown, 0, false, cfg, searchCols)
	if s.TitleOverride != "" {
		t.Errorf("title still says %q after a key", s.TitleOverride)
	}
}

func TestNoticeClearsOnClick(t *testing.T) {
	s, _, cfg := startState(t)
	s.SetNotice("no folder X", 2)
	ClickUnifiedRowResult(s, 0, cfg, 20)
	if s.TitleOverride != "" {
		t.Errorf("title still says %q after a click", s.TitleOverride)
	}
}

// A title set as a message (a command's result) stays through keys; only a
// notice goes at the next one.
func TestTitleMessageOutlivesAKey(t *testing.T) {
	s, searchCols, cfg := startState(t)
	s.SetNotice("no folder X", 2)
	s.SetTitle("ran it", 1)
	HandleUnifiedKeyResult(s, tcell.KeyDown, 0, false, cfg, searchCols)
	if s.TitleOverride != "ran it" {
		t.Errorf("title = %q after a key, want the message kept", s.TitleOverride)
	}
}
