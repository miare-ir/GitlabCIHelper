package main

import (
	"fmt"
	"os"

	"gitlab_ci_helper/internal/setup"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup":
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to resolve working directory: %v\n", err)
			os.Exit(1)
		}
		if err := setup.Run(os.Stdout, os.Stdin, cwd); err != nil {
			fmt.Fprintf(os.Stderr, "setup failed: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("gitlab-ci-helper")
	fmt.Println("Interactive GitLab CI helper setup tool")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  gitlab-ci-helper setup")
	fmt.Println("  gitlab-ci-helper help")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  setup    Run the interactive setup wizard and preview planned changes")
	fmt.Println("  help     Show this help message")
	fmt.Println("")
	fmt.Println("Run from the repository root where .gitlab-ci.yml exists.")
}
