package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"regclean/internal/clean"
	"regclean/internal/scan"
)

// newPrompter returns a clean.Cleaner.Confirm that asks on in/out.
// End of input counts as "abort" so a non-interactive run cannot loop or delete.
func newPrompter(in io.Reader, out io.Writer) func(scan.Entry) clean.Decision {
	r := bufio.NewReader(in)
	return func(e scan.Entry) clean.Decision {
		fmt.Fprintf(out, "\n%s\n  missing: %s\nDelete? [y]es / [n]o / [a]ll / [q]uit: ", describe(e), e.Verdict.Target)
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			fmt.Fprintln(out)
			return clean.Abort
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return clean.Remove
		case "a", "all":
			return clean.RemoveAll
		case "q", "quit":
			return clean.Abort
		}
		return clean.Skip
	}
}
