package index

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type IndexEntry struct {
	Path        string
	Hash        string
	Size        int64
	ModTime     time.Time
	Permissions os.FileMode
}

type Index struct {
	entries   map[string]*IndexEntry
	indexPath string
}

func NewIndex(gitDir string) *Index {
	return &Index{
		entries:   make(map[string]*IndexEntry),
		indexPath: filepath.Join(gitDir, "index"),
	}
}

// Load the index file and populate the entries map
func (idx *Index) Load() error {
	fmt.Printf("DEBUG: Loading index from: %s\n", idx.indexPath)

	file, err := os.Open(idx.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("DEBUG: Index file does not exist, starting with empty index\n")
			return nil
		}
		return fmt.Errorf("failed to open index file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\x00")
		if len(parts) != 5 {
			fmt.Printf("DEBUG: Skipping malformed line with %d parts\n", len(parts))
			continue
		}

		path := parts[0]
		hash := parts[1]
		size, _ := strconv.ParseInt(parts[2], 10, 64)
		modTimeUnix, _ := strconv.ParseInt(parts[3], 10, 64)
		permsInt, _ := strconv.ParseUint(parts[4], 8, 32) // Corrected base to 8 for octal

		fmt.Printf("DEBUG: Loaded entry - Path: '%s', Hash: '%s' (len: %d), Size: %d\n",
			path, hash, len(hash), size)

		idx.entries[path] = &IndexEntry{
			Path:        path,
			Hash:        hash,
			Size:        size,
			ModTime:     time.Unix(modTimeUnix, 0),
			Permissions: os.FileMode(permsInt),
		}
	}

	fmt.Printf("DEBUG: Loaded %d entries from index\n", len(idx.entries))
	return scanner.Err()
}

// Save the index to file with full metadata
func (idx *Index) Save() error {
	fmt.Printf("DEBUG: Saving index to: %s\n", idx.indexPath)
	fmt.Printf("DEBUG: Saving %d entries\n", len(idx.entries))

	file, err := os.Create(idx.indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for path, entry := range idx.entries {
		// Use null character as a separator for robustness
		line := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%o\n",
			entry.Path,
			entry.Hash,
			entry.Size,
			entry.ModTime.Unix(),
			entry.Permissions)

		fmt.Printf("DEBUG: Writing index line for '%s': '%s'\n", path, strings.TrimSpace(line))

		if _, err := writer.WriteString(line); err != nil {
			return fmt.Errorf("failed to write to index file: %w", err)
		}
	}

	fmt.Printf("DEBUG: Index saved successfully\n")
	return nil
}

// Add a file to the index
func (idx *Index) Add(path, hash string, info os.FileInfo) {
	fmt.Printf("DEBUG: Adding to index - Path: '%s', Hash: '%s' (len: %d)\n", path, hash, len(hash))

	if len(hash) != 40 {
		fmt.Printf("WARNING: Invalid hash length for '%s': expected 40, got %d\n", path, len(hash))
	}

	idx.entries[path] = &IndexEntry{
		Path:        path,
		Hash:        hash,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		Permissions: info.Mode(),
	}

	fmt.Printf("DEBUG: Entry added successfully\n")
}

// Remove a file from the index
func (idx *Index) Remove(path string) {
	delete(idx.entries, path)
}

// Get a specific entry by path
func (idx *Index) Get(path string) (*IndexEntry, bool) {
	entry, exists := idx.entries[path]
	return entry, exists
}

// Get all tracked entries
func (idx *Index) GetAll() map[string]*IndexEntry {
	fmt.Printf("DEBUG: GetAll() called, returning %d entries:\n", len(idx.entries))
	for path, entry := range idx.entries {
		fmt.Printf("  - Path: '%s', Hash: '%s' (len: %d)\n", path, entry.Hash, len(entry.Hash))
	}
	return idx.entries
}
