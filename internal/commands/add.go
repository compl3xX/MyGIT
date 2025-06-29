package commands

import (
	"fmt"
	"io/fs"
	"mygit/internal/index"
	"mygit/internal/objects"
	"mygit/internal/repository"
	"os"
	"path/filepath"
	"strings"
)

func Add(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mygit add <file>...")
		os.Exit(1)
	}

	// Find Repo
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

	fmt.Printf("DEBUG: Repository found at: %s\n", repo.GitDir)

	//Init objects store and index
	objStore := objects.NewObjectStore(repo.GitDir)
	idx := index.NewIndex(repo.GitDir)

	fmt.Printf("DEBUG: Loading existing index...\n")
	if err := idx.Load(); err != nil {
		fmt.Printf("Error loading index: %v\n", err)
		os.Exit(1)
	}

	//Process each argument
	for _, arg := range args {
		fmt.Printf("DEBUG: Processing argument: '%s'\n", arg)
		if err := addPath(repo, objStore, idx, arg); err != nil {
			fmt.Printf("Error adding %s: %v\n", arg, err)
			os.Exit(1)
		}
	}

	// Save the index
	fmt.Printf("DEBUG: Saving index...\n")
	if err := idx.Save(); err != nil {
		fmt.Printf("Error saving index: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("DEBUG: Add command completed successfully\n")
}

func addPath(repo *repository.GitRepository, objStore *objects.ObjectStore, idx *index.Index, path string) error {
	fmt.Printf("DEBUG: addPath called with: '%s'\n", path)

	// convert to absolute path if needed
	if !filepath.IsAbs(path) {
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, path)
	}

	fmt.Printf("DEBUG: Absolute path: '%s'\n", path)

	//Get file info
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return addDirectory(repo, objStore, idx, path)
	}

	return addFile(repo, objStore, idx, path, info)
}

func addFile(repo *repository.GitRepository, objStore *objects.ObjectStore, idx *index.Index, path string, info os.FileInfo) error {
	fmt.Printf("DEBUG: addFile called for: '%s'\n", path)

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	fmt.Printf("DEBUG: File content read, length: %d bytes\n", len(content))

	//create blob object
	hash, err := objStore.WriteObject(content, objects.BlobType)
	if err != nil {
		return fmt.Errorf("cannot write object: %w", err)
	}

	fmt.Printf("DEBUG: Object written, hash: '%s' (length: %d)\n", hash, len(hash))

	// Validate hash format
	if len(hash) != 40 {
		return fmt.Errorf("WriteObject returned invalid hash length: expected 40, got %d (hash: '%s')", len(hash), hash)
	}

	//Get relative path from repository root
	relPath, err := filepath.Rel(repo.WorkDir, path)
	if err != nil {
		return fmt.Errorf("cannot get relative path: %w", err)
	}

	relPath = filepath.ToSlash(relPath)
	fmt.Printf("DEBUG: Relative path: '%s'\n", relPath)

	idx.Add(relPath, hash, info)

	fmt.Printf("Added '%s' (hash: %s)\n", relPath, hash[:8])
	return nil
}

func addDirectory(repo *repository.GitRepository, objStore *objects.ObjectStore, idx *index.Index, dirPath string) error {
	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .mygit directory
		if strings.Contains(path, ".mygit") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories themselves, only process files
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		return addFile(repo, objStore, idx, path, info)
	})
}
