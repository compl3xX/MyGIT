package main

import (
	"fmt"
	"mygit/internal/commands"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mygit <command> [args...]")
		fmt.Println("Commands: init, add, commit, log, status, diff, branch, checkout, merge")
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "init":
		commands.Init(args)
	case "add":
		commands.Add(args)
	case "commit":
		commands.Commit(args)
	case "log":
		commands.Log(args)
	case "status":
		commands.Status(args)
	case "push":
		commands.Push(args)
	case "branch":
		commands.Branch(args)
	case "checkout":
		commands.Checkout(args)
	case "show":
		commands.Show(args)
	case "config":
		commands.Config(args)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
