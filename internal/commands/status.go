package commands

import (
	"fmt"
	"mygit/internal/index"
	"mygit/internal/objects"
	"mygit/internal/refs"
	"mygit/internal/repository"
	"mygit/internal/utils"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	refManager := refs.NewRefManager(repo.GitDir)

	// Get current branch name
	currentBranch, err := refManager.GetCurrentBranch()
	if err != nil || currentBranch == "" {
		fmt.Println("On branch main") // Default fallback
	} else {
		fmt.Printf("On branch %s\n", currentBranch)
	}

	// Get HEAD commit and its tree
	headCommitHash, err := refManager.GetHEAD()
	var headTreeEntries map[string]*index.IndexEntry

	if err != nil || headCommitHash == "" {
		// No commits yet, so everything in index is staged
		headTreeEntries = make(map[string]*index.IndexEntry)
	} else {
		// Get the tree from HEAD commit
		headTreeEntries, err = utils.GetTreeEntriesFromCommit(objStore, headCommitHash)
		if err != nil {
			fmt.Printf("Error reading HEAD commit tree: %v\n", err)
			headTreeEntries = make(map[string]*index.IndexEntry)
		}
	}

	indexEntries := idx.GetAll()

	// Find staged changes (index vs HEAD)
	stagedFiles := make([]string, 0)
	for path, indexEntry := range indexEntries {
		if headEntry, exists := headTreeEntries[path]; !exists {
			// New file
			stagedFiles = append(stagedFiles, fmt.Sprintf("new file:   %s", path))
		} else if indexEntry.Hash != headEntry.Hash {
			// Modified file
			stagedFiles = append(stagedFiles, fmt.Sprintf("modified:   %s", path))
		}
	}

	// Check for deleted files (in HEAD but not in index)
	for path := range headTreeEntries {
		if _, exists := indexEntries[path]; !exists {
			stagedFiles = append(stagedFiles, fmt.Sprintf("deleted:    %s", path))
		}
	}

	if len(stagedFiles) > 0 {
		fmt.Println("Changes to be committed:")
		fmt.Println("  (use \"mygit reset HEAD <file>...\" to unstage)")
		fmt.Println()
		for _, fileStatus := range stagedFiles {
			fmt.Printf("        %s\n", fileStatus)
		}
		fmt.Println()
	}

	// Check for modified files (working directory vs index)
	modifiedFiles, err := utils.GetUnstagedChanges(repo, idx, objStore)
	if err != nil {
		fmt.Printf("Error checking for unstaged changes: %v\n", err)
	}

	if len(modifiedFiles) > 0 {
		fmt.Println("Changes not staged for commit:")
		fmt.Println("  (use \"mygit add <file>...\" to update what will be committed)")
		fmt.Println("  (use \"mygit checkout -- <file>...\" to discard changes in working directory)")
		fmt.Println()
		for _, fileStatus := range modifiedFiles {
			fmt.Printf("        modified:   %s\n", fileStatus)
		}
		fmt.Println()
	}

	// Check for untracked files with improved filtering
	untrackedFiles := make([]string, 0)
	err = filepath.Walk(repo.WorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .mygit directory and its contents
		if strings.Contains(path, ".mygit") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip .git directory and its contents (if it exists)
		if strings.Contains(path, ".git") && info.IsDir() {
			return filepath.SkipDir
		}

		// Skip other common ignore patterns
		if shouldIgnoreFile(info.Name()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories themselves (we only care about files)
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(repo.WorkDir, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		// Check if file is tracked in either index or HEAD
		_, trackedInIndex := indexEntries[relPath]
		_, trackedInHead := headTreeEntries[relPath]

		if !trackedInIndex && !trackedInHead {
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

	if len(stagedFiles) == 0 && len(modifiedFiles) == 0 && len(untrackedFiles) == 0 {
		fmt.Println("nothing to commit, working tree clean")
	}
}

// Helper function to check if a file should be ignored
func shouldIgnoreFile(filename string) bool {
	ignoredPatterns := []string{
		".DS_Store",  // macOS
		"Thumbs.db",  // Windows
		".gitignore", // Usually want to track this, but could be configurable
		"*.tmp",
		"*.swp",
		".vscode",
		"node_modules",
	}

	for _, pattern := range ignoredPatterns {
		if strings.Contains(pattern, "*") {
			// Simple wildcard matching
			if strings.HasSuffix(filename, strings.TrimPrefix(pattern, "*")) {
				return true
			}
		} else if filename == pattern {
			return true
		}
	}
	return false
}

// Helper function to extract tree entries from a commit
func getTreeEntriesFromCommit(objStore *objects.ObjectStore, commitHash string) (map[string]*index.IndexEntry, error) {
	// Read commit object
	commitContent, err := objStore.ReadObject(commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read commit %s: %v", commitHash, err)
	}

	// Parse commit to get tree hash
	commit, err := objects.ParseCommit(commitContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commit: %v", err)
	}

	// Get tree entries
	return getTreeEntriesRecursive(objStore, commit.Tree, "")
}

// Helper function to recursively get all entries from a tree
func getTreeEntriesRecursive(objStore *objects.ObjectStore, treeHash, prefix string) (map[string]*index.IndexEntry, error) {
	entries := make(map[string]*index.IndexEntry)

	// Read tree object
	treeContent, err := objStore.ReadObject(treeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read tree %s: %v", treeHash, err)
	}

	// Parse tree entries
	tree, err := objects.ParseTree(treeContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tree: %v", err)
	}

	// Get entries from the tree object
	treeEntries := tree.Entries
	for _, entry := range treeEntries {
		fullPath := entry.Name
		if prefix != "" {
			fullPath = prefix + "/" + entry.Name
		}

		if entry.Mode == "40000" { // Directory
			// Recursively get entries from subdirectory
			subEntries, err := getTreeEntriesRecursive(objStore, entry.Hash, fullPath)
			if err != nil {
				return nil, err
			}
			for path, subEntry := range subEntries {
				entries[path] = subEntry
			}
		} else { // File
			// Create an index entry for comparison
			entries[fullPath] = &index.IndexEntry{
				Path: fullPath,
				Hash: entry.Hash,
				// Note: We don't have size/time info from tree, but hash comparison is sufficient
				// Set default values to avoid comparison issues
				Size:        0,           // Size not available from tree
				ModTime:     time.Time{}, // ModTime not available from tree
				Permissions: 0644,        // Default file permissions
			}
		}
	}

	return entries, nil
}
