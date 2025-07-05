package utils

import (
	"fmt"
	"mygit/internal/index"
	"mygit/internal/objects"
	"mygit/internal/repository"
	"os"
	"path/filepath"
)

// GetUnstagedChanges compares the index with the working directory and returns a list of
// file paths that have been modified or deleted in the working dir but not staged.
func GetUnstagedChanges(repo *repository.GitRepository, idx *index.Index, objStore *objects.ObjectStore) ([]string, error) {
	var modifiedFiles []string

	for path, entry := range idx.GetAll() {
		fullPath := filepath.Join(repo.WorkDir, path)

		_, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				// File is in the index but not in the working directory -> deleted.
				modifiedFiles = append(modifiedFiles, path)
			}
			// For other errors, we might want to report them, but for now, we skip.
			continue
		}

		// This is the most reliable check. We read the file's current content and hash it.
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s for status check: %w", fullPath, err)
		}

		currentHash := objStore.HashObject(content, objects.BlobType)

		// If the hash is different, the file is modified.
		if currentHash != entry.Hash {
			modifiedFiles = append(modifiedFiles, path)
		}
	}

	return modifiedFiles, nil
}
