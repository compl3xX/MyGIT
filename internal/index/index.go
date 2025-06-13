package index

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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
func (idx *Index) Load() error {
	file, err := os.Open(idx.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open index file: %w", err)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		hash := parts[0]
		path := parts[1]

		fullPath := path
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(filepath.Dir(idx.indexPath), "..", path)
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		idx.entries[path] = &IndexEntry{
			Path:        path,
			Hash:        hash,
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			Permissions: info.Mode(),
		}

	}
	return scanner.Err()
}

func (idx *Index) Save() error {
	file, err := os.Create(idx.indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer file.Close()

	for _, entry := range idx.entries {
		line := fmt.Sprintf("%s %s\n", entry.Path, entry.Hash)
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write to index file: %w", err)
		}
	}

	return nil
}

func (idx *Index) Add(path, hash string, info os.FileInfo) {
	idx.entries[path] = &IndexEntry{
		Path:        path,
		Hash:        hash,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		Permissions: info.Mode(),
	}
}

func (idx *Index) Remove(path string) {
	delete(idx.entries, path)
}

func (idx *Index) Get(path string) (*IndexEntry, bool) {
	entry, exists := idx.entries[path]
	return entry, exists
}

func (idx *Index) GetAll() map[string]*IndexEntry {
	return idx.entries
}
