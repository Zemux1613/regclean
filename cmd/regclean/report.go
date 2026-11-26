package main

import (
	"fmt"
	"io"

	"regclean/internal/clean"
	"regclean/internal/pathcheck"
	"regclean/internal/scan"
)

func describe(e scan.Entry) string {
	return fmt.Sprintf("[%s] %-9s %s", e.Hive, e.Area, e.Title)
}

type reporter struct {
	w       io.Writer
	verbose bool
}

func (r reporter) entries(entries []scan.Entry) {
	for _, e := range entries {
		switch e.Verdict.Status {
		case pathcheck.Orphaned:
			fmt.Fprintf(r.w, "%s\n          -> missing: %s\n", describe(e), e.Verdict.Target)
		case pathcheck.Unknown:
			if r.verbose {
				fmt.Fprintf(r.w, "  ?  %s (%s)\n", describe(e), e.Verdict.Note)
			}
		case pathcheck.OK:
			if r.verbose {
				fmt.Fprintf(r.w, "  ok %s\n", describe(e))
			}
		}
	}
}

func (r reporter) scanSummary(s scan.Summary) {
	fmt.Fprintf(r.w, "\n%d Entries checked, %d undecidable (are never touched), %d abandoned.\n",
		s.Checked, s.Unknown, s.Orphaned)
}

func (r reporter) cleanResult(res clean.Result) {
	switch res.Action {
	case clean.Removed:
		fmt.Fprintf(r.w, "  deleted: %s\n", describe(res.Entry))
	case clean.Failed:
		fmt.Fprintf(r.w, "  ERROR at %s: %v\n", describe(res.Entry), res.Err)
	default:
		panic("unhandled default case")
	}
}
