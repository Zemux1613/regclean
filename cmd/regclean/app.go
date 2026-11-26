package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"regclean/internal/clean"
	"regclean/internal/pathcheck"
	"regclean/internal/reg"
	"regclean/internal/scan"
)

const (
	exitOK          = 0
	exitFailure     = 1
	exitUsage       = 2
	exitInterrupted = 130
)

// deps are the things that differ between the real run and tests.
type deps struct {
	store    reg.Store
	fs       pathcheck.FS
	elevated bool
	home     string           // base for the default backup folder
	now      func() time.Time // timestamp for the default backup folder
}

func run(ctx context.Context, args []string, d deps, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseFlags(args, stderr)
	switch {
	case errors.Is(err, flag.ErrHelp):
		return exitOK
	case err != nil:
		fmt.Fprintln(stderr, "Error:", err, "(regclean -h)")
		return exitUsage
	}

	scanner := scan.New(d.store, pathcheck.New(d.fs))
	entries, err := scanner.Scan(ctx, cfg.hives(), scan.LocationsFor(cfg.areas))
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "Aborted.")
		return exitInterrupted
	}
	if err != nil {
		fmt.Fprintf(stderr, "Warning: Not all keys were readable:\n%v\n", err)
	}

	rep := reporter{w: stdout, verbose: cfg.verbose}
	rep.entries(entries)
	rep.scanSummary(scan.Summarize(entries))

	orphans := scan.Orphans(entries)
	switch {
	case len(orphans) == 0:
		return exitOK
	case !cfg.delete:
		fmt.Fprintln(stdout, "Dry run: nothing deleted. Clean up using -delete (including backup).")
		return exitOK
	}
	return cleanup(ctx, cfg, d, orphans, stdin, stdout, stderr, rep)
}

func cleanup(ctx context.Context, cfg config, d deps, orphans []scan.Entry,
	stdin io.Reader, stdout, stderr io.Writer, rep reporter) int {

	todo, skipped := removable(orphans, d.elevated)
	if skipped > 0 {
		fmt.Fprintf(stdout, "\nHint: %d HKLM entries are skipped (start Terminal as an administrator).\n", skipped)
	}
	if len(todo) == 0 {
		return exitOK
	}

	dir := cfg.backupDir
	if dir == "" {
		dir = filepath.Join(d.home, "regclean-backup", d.now().Format("2006-01-02_15-04-05"))
	}
	c := &clean.Cleaner{Store: d.store, BackupDir: dir}
	if !cfg.yes {
		c.Confirm = newPrompter(stdin, stdout)
	}
	fmt.Fprintln(stdout, "\nBackups are stored in:", dir)

	sum, err := c.Run(ctx, todo, rep.cleanResult)
	switch {
	case errors.Is(err, context.Canceled):
		fmt.Fprintf(stderr, "Aborted. %d deleted, %d failed.\n", sum.Removed, sum.Failed)
		return exitInterrupted
	case err != nil:
		fmt.Fprintln(stderr, "Fehler:", err)
		return exitFailure
	}

	if sum.Aborted {
		fmt.Fprintln(stdout, "\nAborted.")
	}
	fmt.Fprintf(stdout, "\n%d deleted, %d skipped, %d failed.\n", sum.Removed, sum.Skipped, sum.Failed)
	if sum.Removed > 0 {
		fmt.Fprintln(stdout, "Restore: Double-click the .reg file in the backup folder or run `reg import <file>`.")
	}
	if sum.Failed > 0 {
		return exitFailure
	}
	return exitOK
}

// removable drops the entries that need administrator rights if we have none.
func removable(entries []scan.Entry, elevated bool) (todo []scan.Entry, skipped int) {
	for _, e := range entries {
		if e.Hive.NeedsAdmin() && !elevated {
			skipped++
			continue
		}
		todo = append(todo, e)
	}
	return todo, skipped
}
