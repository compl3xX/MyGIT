package objects

import (
	"bytes"
	"fmt"
	"mygit/internal/index"
	"path/filepath"
	"sort"
	"strings"
)

type TreeEntry struct {
	Mode string
	Name string
	Hash string
	Type ObjectType
}

type Tree struct {
	Entries []TreeEntry
}

func NewTree() *Tree {
	return &Tree{
		Entries: make([]TreeEntry, 0),
	}
}

func (t *Tree) AddEntry(mode, name, hash string, objType ObjectType) {
	t.Entries = append(t.Entries, TreeEntry{
		Mode: mode,
		Name: name,
		Hash: hash,
		Type: objType,
	})
}

func (t *Tree) Serialize() []byte {
	sort.Slice(t.Entries, func(i, j int) bool {
		return t.Entries[i].Name < t.Entries[j].Name
	})

	var buf bytes.Buffer

	for _, entry := range t.Entries {
		// Format: <mode> <name>\0<20-byte binary hash>
		buf.WriteString(fmt.Sprintf("%s %s\x00", entry.Mode, entry.Name))

		// Convert hex hash to binary
		hashBytes := make([]byte, 20)

		for i := 0; i < 20; i++ {
			fmt.Sscanf(entry.Hash[i*2:i*2+2], "%02x", &hashBytes[i])
		}

		buf.Write(hashBytes)
	}
	return buf.Bytes()
}

func ParseTree(content []byte) (*Tree, error) {
	tree := NewTree()
	offset := 0

	for offset < len(content) {
		// Find space separator
		spaceIdx := bytes.IndexByte(content[offset:], ' ')
		if spaceIdx == -1 {
			break
		}
		spaceIdx += offset

		// Find null terminator
		nullIdx := bytes.IndexByte(content[spaceIdx+1:], 0)
		if nullIdx == -1 {
			break
		}
		nullIdx += spaceIdx + 1

		// Extract mode and name
		mode := string(content[offset:spaceIdx])
		name := string(content[spaceIdx+1 : nullIdx])

		// Extract 20-byte hash
		if nullIdx+21 > len(content) {
			break
		}
		hashBytes := content[nullIdx+1 : nullIdx+21]

		// Convert binary hash to hex
		hash := fmt.Sprintf("%x", hashBytes)

		// Determine object type based on mode
		var objType ObjectType
		if mode == "100644" || mode == "100755" {
			objType = BlobType
		} else if mode == "40000" {
			objType = TreeType
		} else {
			objType = BlobType // Default
		}

		tree.AddEntry(mode, name, hash, objType)
		offset = nullIdx + 21
	}
	return tree, nil
}

func (os *ObjectStore) BuildTreeFromIndex(indexEntries map[string]*index.IndexEntry) (string, error) {
	// Build tree structure from flat index
	treeMap := make(map[string]*Tree)

	// Create root tree
	treeMap[""] = NewTree()

	// Process each index entry
	for path, entry := range indexEntries {
		parts := strings.Split(filepath.ToSlash(path), "/")

		// Create intermediate directories if needed
		currentPath := ""
		for i := 0; i < len(parts)-1; i++ {
			parentPath := currentPath
			if currentPath == "" {
				currentPath = parts[i]
			} else {
				currentPath = currentPath + "/" + parts[i]
			}

			// Create tree for this directory if it doesn't exist
			if _, exists := treeMap[currentPath]; !exists {
				treeMap[currentPath] = NewTree()
			}

			// Add this directory to its parent tree
			if parentTree, exists := treeMap[parentPath]; exists {
				// Check if already added
				found := false
				for _, e := range parentTree.Entries {
					if e.Name == parts[i] && e.Type == TreeType {
						found = true
						break
					}
				}
				if !found {
					parentTree.AddEntry("40000", parts[i], "", TreeType) // Hash will be filled later
				}
			}
		}

		// Add file to its parent directory
		fileName := parts[len(parts)-1]
		parentPath := ""
		if len(parts) > 1 {
			parentPath = strings.Join(parts[:len(parts)-1], "/")
		}

		if parentTree, exists := treeMap[parentPath]; exists {
			mode := "100644"
			if entry.Permissions&0111 != 0 {
				mode = "100755"
			}
			parentTree.AddEntry(mode, fileName, entry.Hash, BlobType)
		}
	}

	// Write trees to object store (bottom-up)
	treeHashes := make(map[string]string)

	// Sort paths by depth (deepest first)
	var paths []string
	for path := range treeMap {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		return strings.Count(paths[i], "/") > strings.Count(paths[j], "/")
	})

	// Write each tree
	for _, path := range paths {
		tree := treeMap[path]

		// Fill in subtree hashes
		for i, entry := range tree.Entries {
			if entry.Type == TreeType && entry.Hash == "" {
				subPath := path
				if subPath == "" {
					subPath = entry.Name
				} else {
					subPath = subPath + "/" + entry.Name
				}
				if hash, exists := treeHashes[subPath]; exists {
					tree.Entries[i].Hash = hash
				}
			}
		}

		// Serialize and write tree
		treeContent := tree.Serialize()
		hash, err := os.WriteObject(treeContent, TreeType)
		if err != nil {
			return "", fmt.Errorf("failed to write tree object: %w", err)
		}

		treeHashes[path] = hash
	}

	// Return root tree hash
	return treeHashes[""], nil
}
