package main

import (
	"fmt"
	"os"

	"github.com/openbpl/openbpl/internal/tui"
)

func main() {
	if err := tui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "openbpl: %v\n", err)
		os.Exit(1)
	}
}
