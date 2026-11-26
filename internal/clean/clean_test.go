package clean_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"regclean/internal/clean"
	"regclean/internal/pathcheck"
	"regclean/internal/pathcheck/pathchecktest"
	"regclean/internal/reg"
	"regclean/internal/reg/regtest"
	"regclean/internal/scan"
)

const (
	cv      = `Software\Microsoft\Windows\CurrentVersion`
	runKey  = cv + `\Run`
	goneKey = cv + `\Uninstall\Gone`
)

var hkcu = reg.Hive{Root: reg.HKCU}

// setup builds a registry with three dead Run values and one dead Uninstall key.
func setup(t *testing.T) (*regtest.Store, []scan.Entry) {
	t.Helper()
	s := regtest.New()
	s.Set(hkcu, runKey, "A", `C:\gone\a.exe`)
	s.Set(hkcu, runKey, "B", `C:\gone\b.exe`)
	s.Set(hkcu, runKey, "C", `C:\gone\c.exe`)
	s.Set(hkcu, goneKey, "UninstallString", `C:\gone\unins.exe`)

	entries, err := scan.New(s, pathcheck.New(pathchecktest.New())).
		Scan(context.Background(), []reg.Hive{hkcu}, scan.LocationsFor(scan.AreaNames()))
	if err != nil {
		t.Fatal(err)
	}
	orphans := scan.Orphans(entries)
	if len(orphans) != 4 {
		t.Fatalf("setup: %d orphans, want 4", len(orphans))
	}
	return s, orphans
}

func TestRemovesEverythingWithoutConfirm(t *testing.T) {
	s, orphans := setup(t)
	c := &clean.Cleaner{Store: s, BackupDir: t.TempDir()}

	sum, err := c.Run(context.Background(), orphans, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Removed != 4 || sum.Failed != 0 {
		t.Errorf("summary = %+v", sum)
	}
	for _, n := range []string{"A", "B", "C"} {
		if s.HasValue(hkcu, runKey, n) {
			t.Errorf("value %s still there", n)
		}
	}
	if s.HasKey(hkcu, goneKey) {
		t.Error("uninstall key still there")
	}
	// one export for the Run key (3 values) + one for the Uninstall key
	if len(s.Exports) != 2 {
		t.Errorf("exports = %d, want 2: %+v", len(s.Exports), s.Exports)
	}
	for _, e := range s.Exports {
		if _, err := os.Stat(e.File); err != nil {
			t.Errorf("backup file missing: %v", err)
		}
	}
}

func TestNothingIsRemovedWithoutBackup(t *testing.T) {
	s, orphans := setup(t)
	s.ExportErr = errors.New("disk full")
	c := &clean.Cleaner{Store: s, BackupDir: t.TempDir()}

	sum, err := c.Run(context.Background(), orphans, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Failed != 4 || sum.Removed != 0 {
		t.Errorf("summary = %+v", sum)
	}
	if !s.HasValue(hkcu, runKey, "A") || !s.HasKey(hkcu, goneKey) {
		t.Error("entries were removed although the backup failed")
	}

	// A failed export must not be cached: once exports work again, the
	// backup has to happen before the removal.
	s.ExportErr = nil
	sum, _ = c.Run(context.Background(), orphans, nil)
	if sum.Removed != 4 || len(s.Exports) != 2 {
		t.Errorf("retry: summary = %+v, exports = %d", sum, len(s.Exports))
	}
}

func TestConfirmDecisions(t *testing.T) {
	s, orphans := setup(t)
	asked := 0
	c := &clean.Cleaner{
		Store:     s,
		BackupDir: t.TempDir(),
		Confirm: func(scan.Entry) clean.Decision {
			asked++
			if asked == 1 {
				return clean.Skip
			}
			return clean.RemoveAll
		},
	}
	var results []clean.Result
	sum, err := c.Run(context.Background(), orphans, func(r clean.Result) { results = append(results, r) })
	if err != nil {
		t.Fatal(err)
	}
	if asked != 2 {
		t.Errorf("asked %d times, want 2 (RemoveAll stops the questions)", asked)
	}
	if sum.Skipped != 1 || sum.Removed != 3 || len(results) != 4 {
		t.Errorf("summary = %+v, results = %d", sum, len(results))
	}
	if !s.HasValue(hkcu, runKey, orphans[0].Value) && orphans[0].Kind == scan.KindValue {
		t.Error("skipped entry was removed")
	}
}

func TestAbort(t *testing.T) {
	s, orphans := setup(t)
	c := &clean.Cleaner{Store: s, BackupDir: t.TempDir(),
		Confirm: func(scan.Entry) clean.Decision { return clean.Abort }}
	sum, err := c.Run(context.Background(), orphans, nil)
	if err != nil || !sum.Aborted || sum.Removed != 0 {
		t.Errorf("summary = %+v, err = %v", sum, err)
	}
}

func TestCancelledContext(t *testing.T) {
	s, orphans := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &clean.Cleaner{Store: s, BackupDir: t.TempDir()}
	if _, err := c.Run(ctx, orphans, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if !s.HasValue(hkcu, runKey, "A") {
		t.Error("removed despite cancelled context")
	}
}
