package commands

import (
	"fmt"
	"mygit/internal/objects"
	"mygit/internal/refs"
	"mygit/internal/repository"
	"os"
	"time"
)

func Log(args []string) {
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

	// Initialize object store and ref manager
	objStore := objects.NewObjectStore(repo.GitDir)
	refManager := refs.NewRefManager(repo.GitDir)

	// Get current commit
	currentCommit, err := refManager.GetHEAD()
	if err != nil || currentCommit == "" {
		fmt.Println("No commits yet")
		return
	}

	// Walk commit history
	commitHash := currentCommit
	for commitHash != "" {
		// Read commit object
		obj, err := objStore.ReadObject(commitHash)
		if err != nil {
			fmt.Printf("Error reading commit %s: %v\n", commitHash, err)
			break
		}

		if obj.Type != objects.CommitType {
			fmt.Printf("Object %s is not a commit\n", commitHash)
			break
		}

		// Parse commit
		commit, err := objects.ParseCommit(obj.Content)
		if err != nil {
			fmt.Printf("Error parsing commit %s: %v\n", commitHash, err)
			break
		}

		// Display commit info
		fmt.Printf("commit %s\n", commitHash)
		fmt.Printf("Author: %s\n", commit.Author)
		fmt.Printf("Date: %s\n", commit.Timestamp.Format(time.RFC3339))
		fmt.Printf("\n    %s\n\n", commit.Message)

		// Move to parent commit
		if len(commit.Parents) > 0 {
			commitHash = commit.Parents[0]
		} else {
			commitHash = ""
		}
	}
}
