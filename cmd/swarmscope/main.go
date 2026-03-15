package main

import (
	"fmt"
	"os"
)

func main() {
	if err := execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
