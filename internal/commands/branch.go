package commands

import (
	"fmt"
	"mygit/internal/refs"
	"mygit/internal/repository"
	"os"
	"path/filepath"
	"strings"
)

// Branch handles the `branch` command logic.
// - If no arguments are provided, it lists all local branches.
// - If one argument is provided, it creates a new branch with that name.
func Branch(args []string) {
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

	// Case 1: List branches
	if len(args) == 0 {
		err := listBranches(refManager)
		if err != nil {
			fmt.Printf("Error listing branches: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Case 2: Create a new branch
	if len(args) == 1 {
		branchName := args[0]
		err := createBranch(refManager, branchName)
		if err != nil {
			fmt.Printf("Error creating branch '%s': %v\n", branchName, err)
			os.Exit(1)
		}
		fmt.Printf("Branch '%s' created.\n", branchName)
		return
	}

	// Case 3: Invalid usage
	fmt.Println("Usage: mygit branch [<branch-name>]")
	os.Exit(1)
}

// listBranches prints all local branches and highlights the current one.
func listBranches(refManager *refs.RefManager) error {
	currentBranch, err := refManager.GetCurrentBranch()
	if err != nil {
		// A detached HEAD is a valid state, but we'll note it.
		// If there's another error, we still want to try listing branches.
		fmt.Println("Note: Not currently on any branch (detached HEAD)")
	}

	headsDir := filepath.Join(refManager.GitDir, "refs", "heads")
	files, err := os.ReadDir(headsDir)
	if err != nil {
		return fmt.Errorf("could not read branches directory: %w", err)
	}

	for _, file := range files {
		branchName := file.Name()
		if strings.HasPrefix(branchName, ".") {
			continue // Skip hidden files
		}

		if branchName == currentBranch {
			fmt.Printf("* %s\n", branchName)
		} else {
			fmt.Printf("  %s\n", branchName)
		}
	}

	return nil
}

// createBranch creates a new branch pointing at the current HEAD commit.
func createBranch(refManager *refs.RefManager, branchName string) error {
	// Check if branch already exists
	newRefPath := filepath.Join("refs", "heads", branchName)
	if _, err := os.Stat(filepath.Join(refManager.GitDir, newRefPath)); err == nil {
		return fmt.Errorf("branch '%s' already exists", branchName)
	}

	// Get the current commit hash from HEAD
	headCommitHash, err := refManager.GetHEAD()
	if err != nil {
		return fmt.Errorf("could not get HEAD commit: %w", err)
	}
	if headCommitHash == "" {
		return fmt.Errorf("cannot create branch from an empty repository with no commits")
	}

	// Create the new ref (branch) pointing to the HEAD commit
	err = refManager.SetRef(newRefPath, headCommitHash)
	if err != nil {
		return fmt.Errorf("failed to write new branch file: %w", err)
	}

	return nil
}
