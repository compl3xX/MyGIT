package commands

import (
	"fmt"
	"mygit/internal/objects"
	"mygit/internal/refs"
	"mygit/internal/repository"
	"os"
	"time"
)

func Show(args []string) {
	// Default to HEAD if no commit specified
	commitHash := "HEAD"
	if len(args) > 0 {
		commitHash = args[0]
	}

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

	// Initialize object store
	objStore := objects.NewObjectStore(repo.GitDir)

	// Resolve commit hash
	if commitHash == "HEAD" {
		refManager := refs.NewRefManager(repo.GitDir)
		resolvedHash, err := refManager.GetHEAD()
		if err != nil || resolvedHash == "" {
			fmt.Println("No commits yet")
			return
		}
		commitHash = resolvedHash
	}

	// Read commit object
	obj, err := objStore.ReadObject(commitHash)
	if err != nil {
		fmt.Printf("Error reading commit %s: %v\n", commitHash, err)
		os.Exit(1)
	}

	if obj.Type != objects.CommitType {
		fmt.Printf("Object %s is not a commit\n", commitHash)
		os.Exit(1)
	}

	// Parse commit
	commit, err := objects.ParseCommit(obj.Content)
	if err != nil {
		fmt.Printf("Error parsing commit %s: %v\n", commitHash, err)
		os.Exit(1)
	}

	// Display detailed commit info
	fmt.Printf("commit %s\n", commitHash)
	fmt.Printf("Author: %s\n", commit.Author)
	fmt.Printf("Date: %s\n", commit.Timestamp.Format(time.RFC3339))
	fmt.Printf("\n    %s\n\n", commit.Message)

	// TODO: Show diff (will implement in Phase 5)
	fmt.Printf("Tree: %s\n", commit.Tree)
	if len(commit.Parents) > 0 {
		fmt.Printf("Parent: %s\n", commit.Parents[0])
	}
}
