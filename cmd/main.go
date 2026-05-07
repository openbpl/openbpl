package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/openbpl/openbpl/internal/project"
	"github.com/openbpl/openbpl/internal/tui"
	"github.com/openbpl/openbpl/internal/wizard"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  openbpl create <project-name> [--blank]\n")
	fmt.Fprintf(os.Stderr, "  openbpl start\n")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: openbpl create <project-name> [--blank]\n")
			os.Exit(1)
		}

		// Find project name (first non-flag argument after "create")
		var name string
		args := os.Args[2:]
		for _, arg := range args {
			if arg != "--blank" {
				name = arg
				break
			}
		}
		if name == "" {
			fmt.Fprintf(os.Stderr, "Usage: openbpl create <project-name> [--blank]\n")
			os.Exit(1)
		}

		blank := slices.Contains(args, "--blank")

		var configContent string
		if !blank {
			result, err := wizard.Run()
			if err != nil {
				fmt.Fprintf(os.Stderr, "openbpl: wizard failed: %v\n", err)
				os.Exit(1)
			}
			configContent = wizard.GenerateConfig(result)
		}

		if err := project.Create(name, configContent); err != nil {
			fmt.Fprintf(os.Stderr, "openbpl: %v\n", err)
			os.Exit(1)
		}
	case "start":
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "openbpl: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "openbpl: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}
