package commands

import (
	"fmt"
	"mygit/internal/repository"
	"os"
)

func Init(args []string) {
	var targetDir string

	if len(args) > 0 {
		targetDir = args[0]
	} else {
		var err error
		targetDir, err = os.Getwd()
		if err != nil {
			fmt.Printf("Error getting current directory: %v\n", err)
			os.Exit(1)
		}
	}

	repo := repository.NewGitRepository(targetDir)

	if repo.Exists() {
		fmt.Println("Repository already exists!")
		return
	}

	if err := repo.Init(); err != nil {
		fmt.Printf("Error initializing repository: %v\n", err)
		os.Exit(1)
	}

}
