package main

import (
	"fmt"
	"os"

	"github.com/openbpl/openbpl/internal/project"
	"github.com/openbpl/openbpl/internal/tui"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  openbpl create <project-name>\n")
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
			fmt.Fprintf(os.Stderr, "Usage: openbpl create <project-name>\n")
			os.Exit(1)
		}
		if err := project.Create(os.Args[2]); err != nil {
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
