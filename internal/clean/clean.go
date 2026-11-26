// Package clean removes orphaned entries. Every removal is preceded by a .reg
// backup; if the backup fails, nothing is removed.
package clean

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"regclean/internal/reg"
	"regclean/internal/scan"
)

// Decision is the answer to Cleaner.Confirm.
type Decision int

const (
	Skip      Decision = iota // leave this entry alone
	Remove                    // remove this entry
	RemoveAll                 // remove this and all following entries without asking
	Abort                     // stop immediately
)

// Action is what happened to one entry.
type Action int

const (
	Removed Action = iota
	Skipped
	Failed
)

// Result reports the outcome for one entry.
type Result struct {
	Entry  scan.Entry
	Action Action
	Err    error // set when Action == Failed
}

// Summary counts the outcomes of a Run.
type Summary struct {
	Removed, Skipped, Failed int
	Aborted                  bool
}

// Cleaner removes entries from a Store.
type Cleaner struct {
	Store     reg.Store
	BackupDir string
	// Confirm is asked before each removal; nil means "remove everything".
	Confirm func(scan.Entry) Decision

	exported map[string]bool // keys already backed up by this Cleaner
	seq      int
}

// Run processes entries in order and reports each outcome through report
// (which may be nil). It returns early with ctx.Err() if ctx is cancelled.
func (c *Cleaner) Run(ctx context.Context, entries []scan.Entry, report func(Result)) (Summary, error) {
	var sum Summary
	if len(entries) == 0 {
		return sum, nil
	}
	if err := os.MkdirAll(c.BackupDir, 0o755); err != nil {
		return sum, fmt.Errorf("Create a backup folder: %w", err)
	}
	if report == nil {
		report = func(Result) {}
	}

	askNoMore := c.Confirm == nil
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		if !askNoMore {
			switch c.Confirm(e) {
			case Skip:
				sum.Skipped++
				report(Result{Entry: e, Action: Skipped})
				continue
			case Abort:
				sum.Aborted = true
				return sum, nil
			case RemoveAll:
				askNoMore = true
			default:
				panic("unhandled default case")
			}
		}

		if err := c.removeWithBackup(e); err != nil {
			sum.Failed++
			report(Result{Entry: e, Action: Failed, Err: err})
			continue
		}
		sum.Removed++
		report(Result{Entry: e, Action: Removed})
	}
	return sum, nil
}

func (c *Cleaner) removeWithBackup(e scan.Entry) error {
	if err := c.backup(e); err != nil {
		return fmt.Errorf("Backup failed; nothing was deleted: %w", err)
	}
	switch e.Kind {
	case scan.KindValue:
		return c.Store.DeleteValue(e.Hive, e.Key, e.Value)
	case scan.KindKey:
		return c.Store.DeleteKey(e.Hive, e.Key)
	}
	return fmt.Errorf("Unknown entry type %d", e.Kind)
}

// backup exports the key that is about to be modified. For values that is the
// containing key, which is exported only once however many of its values go.
func (c *Cleaner) backup(e scan.Entry) error {
	id := e.Hive.String() + "|" + strings.ToLower(e.Key)
	if c.exported[id] {
		return nil
	}
	c.seq++
	file := filepath.Join(c.BackupDir, backupName(c.seq, e))
	if err := c.Store.Export(e.Hive, e.Key, file); err != nil {
		return err
	}
	// Only remember successful exports, otherwise a failed one would let
	// later entries of the same key skip their backup.
	if c.exported == nil {
		c.exported = map[string]bool{}
	}
	c.exported[id] = true
	return nil
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func backupName(seq int, e scan.Entry) string {
	name := fmt.Sprintf("%03d_%s_%s", seq, e.Hive, e.Area)
	if e.Kind == scan.KindKey {
		name += "_" + e.Key[strings.LastIndex(e.Key, `\`)+1:]
	}
	name = unsafeChars.ReplaceAllString(name, "_")
	if len(name) > 90 {
		name = name[:90]
	}
	return name + ".reg"
}
