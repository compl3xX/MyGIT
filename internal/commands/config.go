package commands

import (
	"fmt"
	"mygit/internal/config"
	"mygit/internal/repository"
	"os"
	"path/filepath"
)

func Config(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: mygit config [--list | <key> [<value>]]")
		os.Exit(1)
	}

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

	configPath := filepath.Join(repo.GitDir, "config")
	cfg := config.NewConfig(configPath)
	if err := cfg.Load(); err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	switch args[0] {
	case "--list":
		for key, value := range cfg.GetAll() {
			fmt.Printf("%s=%s\n", key, value)
		}
	default:
		key := args[0]
		if len(args) > 1 {
			// Set value
			value := args[1]
			cfg.Set(key, value)
			if err := cfg.Save(); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Get value
			if value, ok := cfg.Get(key); ok {
				fmt.Println(value)
			} else {
				os.Exit(1) // Exit with error if key not found
			}
		}
	}
}
