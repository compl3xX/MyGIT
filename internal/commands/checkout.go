package commands

import (
	"fmt"
	"mygit/internal/index"
	"mygit/internal/objects"
	"mygit/internal/refs"
	"mygit/internal/repository"
	"mygit/internal/utils"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Checkout handles the `checkout` command.
// It can switch the current HEAD to a specified branch.
func Checkout(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: mygit checkout <branch-name>")
		os.Exit(1)
	}
	branchName := args[0]

	// Find the repository
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

	refManager := refs.NewRefManager(repo.GitDir)
	objStore := objects.NewObjectStore(repo.GitDir)
	idx := index.NewIndex(repo.GitDir)

	// --- Full Safety Check ---
	if err := idx.Load(); err != nil {
		fmt.Printf("Error loading index: %v\n", err)
		os.Exit(1)
	}

	// 1. Check for unstaged changes (working directory vs. index)
	unstagedChanges, err := utils.GetUnstagedChanges(repo, idx, objStore)
	if err != nil {
		fmt.Printf("Error checking for unstaged changes: %v\n", err)
		os.Exit(1)
	}
	if len(unstagedChanges) > 0 {
		fmt.Println("error: Your local changes to the following files would be overwritten by checkout:")
		for _, file := range unstagedChanges {
			fmt.Printf("\t%s\n", file)
		}
		fmt.Println("Please commit your changes or stash them before you switch branches.")
		os.Exit(1)
	}

	// 2. Check for staged changes (index vs. HEAD)
	headCommitHash, _ := refManager.GetHEAD()
	var headTree map[string]*index.IndexEntry
	if headCommitHash != "" {
		headTree, _ = utils.GetTreeEntriesFromCommit(objStore, headCommitHash)
	} else {
		headTree = make(map[string]*index.IndexEntry) // Empty repo, empty tree
	}

	if isDirty(idx, headTree) {
		fmt.Println("error: Your local changes would be overwritten by checkout.")
		fmt.Println("Please commit your changes or stash them before you switch branches.")
		os.Exit(1)
	}
	// --- End of Safety Check ---

	// Get the commit hash for the target branch
	targetRef := path.Join("refs", "heads", branchName)
	targetCommitHash, err := refManager.GetRef(targetRef)
	if err != nil || targetCommitHash == "" {
		fmt.Printf("Error: branch '%s' not found.\n", branchName)
		os.Exit(1)
	}

	// Update HEAD to point to the new branch
	if err := refManager.SetHEAD(targetRef); err != nil {
		fmt.Printf("Error updating HEAD: %v\n", err)
		os.Exit(1)
	}

	// Get the tree from the target commit
	commit, err := objStore.ReadObject(targetCommitHash)
	if err != nil || commit.Type != objects.CommitType {
		fmt.Printf("Error reading target commit object: %v\n", err)
		os.Exit(1)
	}
	parsedCommit, _ := objects.ParseCommit(commit.Content)
	treeHash := parsedCommit.Tree

	// Clear the current working directory (of tracked files)
	if err := clearWorkingDirectory(repo, idx); err != nil {
		fmt.Printf("Error clearing working directory: %v\n", err)
		os.Exit(1)
	}

	// Update the index and working directory from the target tree
	newIndex := index.NewIndex(repo.GitDir) // Create a fresh index
	if err := updateWorkspaceFromTree(repo, objStore, newIndex, treeHash, ""); err != nil {
		fmt.Printf("Error updating workspace from tree: %v\n", err)
		os.Exit(1)
	}

	// Save the new index
	if err := newIndex.Save(); err != nil {
		fmt.Printf("Error saving new index: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Switched to branch '%s'\n", branchName)
}

// isDirty checks if there are any differences between the index and HEAD tree.
func isDirty(idx *index.Index, headTree map[string]*index.IndexEntry) bool {
	indexEntries := idx.GetAll()

	// Check for additions or modifications
	for path, indexEntry := range indexEntries {
		if headEntry, exists := headTree[path]; !exists || headEntry.Hash != indexEntry.Hash {
			return true
		}
	}

	// Check for deletions
	for path := range headTree {
		if _, exists := indexEntries[path]; !exists {
			return true
		}
	}

	return false
}

// clearWorkingDirectory removes all files and directories tracked in the index.
func clearWorkingDirectory(repo *repository.GitRepository, idx *index.Index) error {
	// It's safer to load the index again to ensure we have the right file list.
	idx.Load()
	for path := range idx.GetAll() {
		fullPath := filepath.Join(repo.WorkDir, path)
		if err := os.Remove(fullPath); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// updateWorkspaceFromTree recursively populates the index and working dir from a tree.
func updateWorkspaceFromTree(repo *repository.GitRepository, objStore *objects.ObjectStore, idx *index.Index, treeHash, currentPath string) error {
	treeObj, err := objStore.ReadObject(treeHash)
	if err != nil || treeObj.Type != objects.TreeType {
		return fmt.Errorf("could not read tree object %s", treeHash)
	}

	tree, err := objects.ParseTree(treeObj.Content)
	if err != nil {
		return fmt.Errorf("could not parse tree object %s", treeHash)
	}

	for _, entry := range tree.Entries {
		pathInRepo := filepath.Join(currentPath, entry.Name)

		if entry.Type == objects.TreeType {
			// It's a directory, recurse
			if err := updateWorkspaceFromTree(repo, objStore, idx, entry.Hash, pathInRepo); err != nil {
				return err
			}
		} else {
			// It's a file (blob)
			blobObj, err := objStore.ReadObject(entry.Hash)
			if err != nil {
				return fmt.Errorf("could not read blob object %s", entry.Hash)
			}

			// Write the file to the working directory
			filePath := filepath.Join(repo.WorkDir, pathInRepo)
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(filePath, blobObj.Content, 0644); err != nil {
				return err
			}

			// Add the file to the new index
			info, _ := os.Stat(filePath)
			idx.Add(strings.ReplaceAll(pathInRepo, "\\", "/"), entry.Hash, info)
		}
	}

	return nil
}
