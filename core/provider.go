package core

import (
	"os"
	"path/filepath"
	"sort"
)

// TreeProvider loads children for a tree node on demand.
// Implementations determine where data comes from (YAML, filesystem, etc.).
type TreeProvider interface {
	// LoadChildren returns child items for the node at the given path.
	// parentPath is the slash-separated breadcrumb (e.g., "D:/repos/my-homepage").
	// Returns items with Depth, Fields, HasChildren set. ParentIdx is set by the caller.
	LoadChildren(parentPath string) []Item
}

// VolatileProvider is a TreeProvider whose folders describe live state (the
// open windows) that goes stale while a long-lived menu stays open. A folder
// it calls volatile is read again every time it is entered, its old rows
// dropped from the tree.
type VolatileProvider interface {
	TreeProvider
	Volatile(parentPath string) bool
}

// DirProvider loads directory contents from the filesystem.
type DirProvider struct {
	// ExcludeDirs are directory names to skip (e.g., ".git", "node_modules").
	ExcludeDirs map[string]bool
}

// NewDirProvider creates a DirProvider with common exclusions.
func NewDirProvider() *DirProvider {
	return &DirProvider{
		ExcludeDirs: map[string]bool{
			".git":          true,
			"node_modules":  true,
			"$Recycle.Bin":  true,
			"System Volume Information": true,
		},
	}
}

// LoadChildren reads a directory and returns its entries as Items.
func (p *DirProvider) LoadChildren(parentPath string) []Item {
	entries, err := os.ReadDir(parentPath)
	if err != nil {
		return nil
	}

	var dirs, files []Item
	for _, entry := range entries {
		name := entry.Name()
		if p.ExcludeDirs[name] {
			continue
		}

		fullPath := filepath.Join(parentPath, name)
		isDir := entry.IsDir()
		// A link or junction reports its own type, not its target's, so a
		// folder reached through one (macOS's /d, /tmp) would read as a file.
		if !isDir && entry.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			if fi, err := os.Stat(fullPath); err == nil {
				isDir = fi.IsDir()
			}
		}
		item := Item{
			Fields:      []string{name},
			HasChildren: isDir,
			ParentIdx:   -1, // set by caller
		}
		if !isDir {
			item.Fields = append(item.Fields, fullPath)
		}

		if isDir {
			dirs = append(dirs, item)
		} else {
			files = append(files, item)
		}
	}

	// Sort: directories first, then files, each alphabetical
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Fields[0] < dirs[j].Fields[0] })
	sort.Slice(files, func(i, j int) bool { return files[i].Fields[0] < files[j].Fields[0] })

	return append(dirs, files...)
}
