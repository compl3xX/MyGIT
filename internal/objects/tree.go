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
	// Sort entries by name as required by Git
	sort.Slice(t.Entries, func(i, j int) bool {
		return t.Entries[i].Name < t.Entries[j].Name
	})

	var buf bytes.Buffer

	validModes := map[string]bool{
		"100644": true,
		"100755": true,
		"040000": true, // Corrected mode for directories
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
		} else if mode == "040000" { // Corrected mode for directories
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
	// Group entries by directory
	rootDir := &treeNode{name: "", children: make(map[string]*treeNode), entries: make([]TreeEntry, 0)}

	for path, entry := range indexEntries {
		parts := strings.Split(filepath.ToSlash(path), "/")
		currentNode := rootDir

		for _, part := range parts[:len(parts)-1] {
			if _, ok := currentNode.children[part]; !ok {
				currentNode.children[part] = &treeNode{name: part, children: make(map[string]*treeNode), entries: make([]TreeEntry, 0)}
			}
			currentNode = currentNode.children[part]
		}

		fileName := parts[len(parts)-1]
		mode := "100644"
		if entry.Permissions&0111 != 0 {
			mode = "100755"
		}
		currentNode.entries = append(currentNode.entries, TreeEntry{Mode: mode, Name: fileName, Hash: entry.Hash, Type: BlobType})
	}

	// Recursively write trees to the object store
	return writeTreeRecursive(os, rootDir)
}

// treeNode represents a node in the tree structure (either a directory or a file)
type treeNode struct {
	name     string
	children map[string]*treeNode
	entries  []TreeEntry
}

// writeTreeRecursive writes a tree and its children to the object store and returns the tree's hash.
func writeTreeRecursive(os *ObjectStore, node *treeNode) (string, error) {
	// First, write all child trees and get their hashes
	for name, childNode := range node.children {
		childHash, err := writeTreeRecursive(os, childNode)
		if err != nil {
			return "", err
		}
		node.entries = append(node.entries, TreeEntry{Mode: "040000", Name: name, Hash: childHash, Type: TreeType}) // Corrected mode
	}

	// Now, create a Tree object from the entries and serialize it
	tree := &Tree{Entries: node.entries}
	treeContent := tree.Serialize() // This already sorts the entries

	// Write the tree object to the store
	return os.WriteObject(treeContent, TreeType)
}
