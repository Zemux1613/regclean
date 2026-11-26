//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "regclean runs only on Windows")
	os.Exit(exitFailure)
}
