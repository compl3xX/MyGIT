package objects

import (
	"bytes"
	"encoding/hex"
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
	var buf bytes.Buffer

	validModes := map[string]bool{
		"100644": true,
		"100755": true,
		"40000":  true,
	}

	for _, entry := range t.Entries {
		if !validModes[entry.Mode] {
			panic(fmt.Sprintf("❌ INVALID MODE '%s' in entry '%s'", entry.Mode, entry.Name))
		}

		if len(entry.Hash) != 40 {
			panic(fmt.Sprintf("❌ INVALID HASH LENGTH for %s: got %d", entry.Name, len(entry.Hash)))
		}

		buf.Write([]byte(entry.Mode))
		buf.WriteByte(' ')
		buf.Write([]byte(entry.Name))
		buf.WriteByte(0)

		hashBytes, err := hex.DecodeString(entry.Hash)
		if err != nil || len(hashBytes) != 20 {
			panic(fmt.Sprintf("❌ HASH DECODE FAILED for %s: %v", entry.Name, err))
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

	// First pass: create all directories and collect all paths
	allDirs := make(map[string]bool)

	for path := range indexEntries {
		parts := strings.Split(filepath.ToSlash(path), "/")

		// Add all directory paths
		currentPath := ""
		for i := 0; i < len(parts)-1; i++ {
			if currentPath == "" {
				currentPath = parts[i]
			} else {
				currentPath = currentPath + "/" + parts[i]
			}
			allDirs[currentPath] = true
		}
	}

	// Create trees for all directories
	for dir := range allDirs {
		treeMap[dir] = NewTree()
	}

	// Second pass: build directory hierarchy
	for dir := range allDirs {
		parts := strings.Split(dir, "/")
		dirName := parts[len(parts)-1]

		// Find parent path
		var parentPath string
		if len(parts) == 1 {
			parentPath = "" // root
		} else {
			parentPath = strings.Join(parts[:len(parts)-1], "/")
		}

		// Add this directory to its parent
		if parentTree, exists := treeMap[parentPath]; exists {
			// Check if already added
			found := false
			for _, e := range parentTree.Entries {
				if e.Name == dirName && e.Type == TreeType {
					found = true
					break
				}
			}
			if !found {
				parentTree.AddEntry("40000", dirName, "PLACEHOLDER", TreeType)
			}
		}
	}

	// Third pass: add files to their parent directories
	for path, entry := range indexEntries {
		parts := strings.Split(filepath.ToSlash(path), "/")
		fileName := parts[len(parts)-1]

		// Find parent directory
		var parentPath string
		if len(parts) == 1 {
			parentPath = "" // root
		} else {
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
		depthI := strings.Count(paths[i], "/")
		depthJ := strings.Count(paths[j], "/")
		if paths[i] == "" {
			depthI = -1 // Root should be last
		}
		if paths[j] == "" {
			depthJ = -1 // Root should be last
		}
		return depthI > depthJ
	})

	fmt.Printf("DEBUG: Processing trees in order: %v\n", paths)

	// Write each tree
	for _, path := range paths {
		tree := treeMap[path]

		fmt.Printf("DEBUG: Processing tree for path '%s' with %d entries\n", path, len(tree.Entries))

		// Fill in subtree hashes
		for i, entry := range tree.Entries {
			if entry.Type == TreeType && entry.Hash == "PLACEHOLDER" {
				subPath := path
				if subPath == "" {
					subPath = entry.Name
				} else {
					subPath = subPath + "/" + entry.Name
				}

				fmt.Printf("DEBUG: Looking for hash for subtree: '%s'\n", subPath)
				if hash, exists := treeHashes[subPath]; exists {
					fmt.Printf("DEBUG: Found hash for '%s': %s\n", subPath, hash)
					tree.Entries[i].Hash = hash
				} else {
					fmt.Printf("DEBUG: Available tree hashes: %v\n", treeHashes)
					return "", fmt.Errorf("missing hash for subtree: %s", subPath)
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
		fmt.Printf("DEBUG: Stored tree hash for path '%s': %s\n", path, hash)
	}

	// Return root tree hash
	rootHash := treeHashes[""]
	fmt.Printf("DEBUG: Returning root tree hash: %s\n", rootHash)
	return rootHash, nil
}
