package utils

import (
	"fmt"
	"mygit/internal/index"
	"mygit/internal/objects"
	"time"
)

// GetTreeEntriesFromCommit reads a commit and returns a map of all files in its tree.
func GetTreeEntriesFromCommit(objStore *objects.ObjectStore, commitHash string) (map[string]*index.IndexEntry, error) {
	commitContent, err := objStore.ReadObject(commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit %s: %v", commitHash, err)
	}

	commit, err := objects.ParseCommit(commitContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commit: %v", err)
	}

	return GetTreeEntriesRecursive(objStore, commit.Tree, "")
}

// GetTreeEntriesRecursive traverses a tree object and returns a map of all files.
func GetTreeEntriesRecursive(objStore *objects.ObjectStore, treeHash, prefix string) (map[string]*index.IndexEntry, error) {
	entries := make(map[string]*index.IndexEntry)

	treeContent, err := objStore.ReadObject(treeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read tree %s: %v", treeHash, err)
	}

	tree, err := objects.ParseTree(treeContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tree: %v", err)
	}

	treeEntries := tree.Entries
	for _, entry := range treeEntries {
		fullPath := entry.Name
		if prefix != "" {
			fullPath = prefix + "/" + entry.Name
		}

		if entry.Mode == "40000" { // Directory
			subEntries, err := GetTreeEntriesRecursive(objStore, entry.Hash, fullPath)
			if err != nil {
				return nil, err
			}
			for path, subEntry := range subEntries {
				entries[path] = subEntry
			}
		} else { // File
			entries[fullPath] = &index.IndexEntry{
				Path:        fullPath,
				Hash:        entry.Hash,
				Size:        0,
				ModTime:     time.Time{},
				Permissions: 0644,
			}
		}
	}

	return entries, nil
}
