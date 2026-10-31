package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirProviderFollowsLinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "dirlink")); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "file.txt"), filepath.Join(dir, "filelink")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "broken")); err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	for _, it := range NewDirProvider().LoadChildren(dir) {
		got[it.Fields[0]] = it.HasChildren
	}
	want := map[string]bool{"real": true, "dirlink": true, "file.txt": false, "filelink": false, "broken": false}
	for name, hasChildren := range want {
		if v, ok := got[name]; !ok || v != hasChildren {
			t.Errorf("%s: HasChildren=%v present=%v, want %v", name, v, ok, hasChildren)
		}
	}
}
