package commands

import (
	"fmt"
	"mygit/internal/index"
	"mygit/internal/objects"
	"mygit/internal/repository"
	"os"
	"path/filepath"
	"strings"
)

func Status(args []string) {
	// Find repository
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	repo, err := repository.FindRepository(cwd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Load index
	idx := index.NewIndex(repo.GitDir)
	if err := idx.Load(); err != nil {
		fmt.Printf("Error loading index: %v\n", err)
		os.Exit(1)
	}

	objStore := objects.NewObjectStore(repo.GitDir)

	fmt.Println("On branch main") // TODO: Get current branch
	fmt.Println()

	// Check for staged files
	indexEntries := idx.GetAll()
	if len(indexEntries) > 0 {
		fmt.Println("Changes to be committed:")
		fmt.Println("  (use \"mygit reset HEAD <file>...\" to unstage)")
		fmt.Println()
		for path := range indexEntries {
			fmt.Printf("        new file:   %s\n", path)
		}
		fmt.Println()
	}

	// Check for modified files
	modifiedFiles := make([]string, 0)
	for path, entry := range indexEntries {
		fullPath := filepath.Join(repo.WorkDir, path)

		// Check if file still exists
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("        deleted:    %s\n", path)
			}
			continue
		}

		// Check if file was modified
		if info.ModTime().After(entry.ModTime) {
			// Read current content and hash it
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			currentHash := objStore.HashObject(content, objects.BlobType)
			if currentHash != entry.Hash {
				modifiedFiles = append(modifiedFiles, path)
			}
		}
	}

	if len(modifiedFiles) > 0 {
		fmt.Println("Changes not staged for commit:")
		fmt.Println("  (use \"mygit add <file>...\" to update what will be committed)")
		fmt.Println()
		for _, path := range modifiedFiles {
			fmt.Printf("        modified:   %s\n", path)
		}
		fmt.Println()
	}

	// Check for untracked files
	untrackedFiles := make([]string, 0)
	err = filepath.Walk(repo.WorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .mygit directory
		if strings.Contains(path, ".mygit") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(repo.WorkDir, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		// Check if file is tracked
		if _, tracked := indexEntries[relPath]; !tracked {
			untrackedFiles = append(untrackedFiles, relPath)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
	}

	if len(untrackedFiles) > 0 {
		fmt.Println("Untracked files:")
		fmt.Println("  (use \"mygit add <file>...\" to include in what will be committed)")
		fmt.Println()
		for _, path := range untrackedFiles {
			fmt.Printf("        %s\n", path)
		}
		fmt.Println()
	}

	if len(indexEntries) == 0 && len(modifiedFiles) == 0 && len(untrackedFiles) == 0 {
		fmt.Println("nothing to commit, working tree clean")
	}
}
