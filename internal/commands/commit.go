package commands

import (
	"bufio"
	"fmt"
	"mygit/internal/config"
	"mygit/internal/index"
	"mygit/internal/objects"
	"mygit/internal/refs"
	"mygit/internal/repository"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func Commit(args []string) {
	var message string

	// Parse arguments
	for i, arg := range args {
		if arg == "-m" && i+1 < len(args) {
			message = args[i+1]
			break
		}
	}

	if message == "" {
		// Get message from user input
		fmt.Print("Enter commit message: ")
		reader := bufio.NewReader(os.Stdin)
		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading message: %v\n", err)
			os.Exit(1)
		}
		message = strings.TrimSpace(msg)
	}

	if message == "" {
		fmt.Println("Commit message cannot be empty")
		os.Exit(1)
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

	// Load index
	idx := index.NewIndex(repo.GitDir)
	if err := idx.Load(); err != nil {
		fmt.Printf("Error loading index: %v\n", err)
		os.Exit(1)
	}

	indexEntries := idx.GetAll()
	if len(indexEntries) == 0 {
		fmt.Println("nothing to commit, working tree clean")
		return
	}

	// Initialize object store and ref manager
	objStore := objects.NewObjectStore(repo.GitDir)
	refManager := refs.NewRefManager(repo.GitDir)

	// Build tree from index
	treeHash, err := objStore.BuildTreeFromIndex(indexEntries)
	if err != nil {
		fmt.Printf("Error building tree: %v\n", err)
		os.Exit(1)
	}

	// Get current commit (parent)
	var parents []string
	currentCommit, err := refManager.GetHEAD()
	if err == nil && currentCommit != "" {
		parents = append(parents, currentCommit)
	}

	// Get author info
	author := getAuthor(repo)

	// Create commit object
	commit := objects.NewCommit(treeHash, message, author, parents)
	commitContent := commit.Serialize()

	commitHash, err := objStore.WriteObject(commitContent, objects.CommitType)
	if err != nil {
		fmt.Printf("Error writing commit object: %v\n", err)
		os.Exit(1)
	}

	// Update current branch
	if err := refManager.UpdateCurrentBranch(commitHash); err != nil {
		fmt.Printf("Error updating branch: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[main %s] %s\n", commitHash[:7], message)
	fmt.Printf(" %d files changed\n", len(indexEntries))
}

func getAuthor(repo *repository.GitRepository) string {
	configPath := filepath.Join(repo.GitDir, "config")
	cfg := config.NewConfig(configPath)
	if err := cfg.Load(); err != nil {
		// Fall back to system user if config can't be loaded
		currentUser, err := user.Current()
		if err != nil {
			return "Unknown User <unknown@example.com>"
		}
		return fmt.Sprintf("%s <%s@localhost>", currentUser.Username, currentUser.Username)
	}

	name, okName := cfg.Get("user.name")
	email, okEmail := cfg.Get("user.email")

	if okName && okEmail {
		return fmt.Sprintf("%s <%s>", name, email)
	}

	// Fall back to system user if config is incomplete
	currentUser, err := user.Current()
	if err != nil {
		return "Unknown User <unknown@example.com>"
	}
	return fmt.Sprintf("%s <%s@localhost>", currentUser.Username, currentUser.Username)
}
