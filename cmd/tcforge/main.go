package main

import (
	"fmt"
	"os"

	"github.com/gguillory-hub/tcforge/internal/tcforge"
)

func main() {
	if err := tcforge.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
