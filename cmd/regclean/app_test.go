package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"regclean/internal/pathcheck/pathchecktest"
	"regclean/internal/reg"
	"regclean/internal/reg/regtest"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

var (
	hkcu   = reg.Hive{Root: reg.HKCU}
	hklm64 = reg.Hive{Root: reg.HKLM, View: reg.View64}
)

type env struct {
	store *regtest.Store
	deps  deps
	home  string
}

func newEnv(t *testing.T, elevated bool) env {
	t.Helper()
	s := regtest.New()
	s.Set(hkcu, runKey, "Alive", `C:\Apps\alive.exe`)
	s.Set(hkcu, runKey, "DeadUser", `C:\Apps\dead-user.exe`)
	s.Set(hklm64, runKey, "DeadMachine", `C:\Apps\dead-machine.exe`)

	home := t.TempDir()
	return env{store: s, home: home, deps: deps{
		store:    s,
		fs:       pathchecktest.New(`C:\Apps\alive.exe`),
		elevated: elevated,
		home:     home,
		now:      func() time.Time { return time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC) },
	}}
}

func (e env) run(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(context.Background(), args, e.deps, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestDryRunChangesNothing(t *testing.T) {
	e := newEnv(t, true)
	code, out, _ := e.run(t, "")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"DeadUser", "DeadMachine", "2 verwaist", "Dry-Run"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !e.store.HasValue(hkcu, runKey, "DeadUser") || !e.store.HasValue(hklm64, runKey, "DeadMachine") {
		t.Error("dry-run modified the registry")
	}
	if len(e.store.Exports) != 0 {
		t.Error("dry-run created backups")
	}
}

func TestDeleteYes(t *testing.T) {
	e := newEnv(t, true)
	code, out, _ := e.run(t, "", "-delete", "-yes")
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if e.store.HasValue(hkcu, runKey, "DeadUser") || e.store.HasValue(hklm64, runKey, "DeadMachine") {
		t.Error("orphans still there")
	}
	if !e.store.HasValue(hkcu, runKey, "Alive") {
		t.Error("intact entry was removed")
	}
	if len(e.store.Exports) != 2 {
		t.Errorf("exports = %d, want 2 (one per key)", len(e.store.Exports))
	}
	wantDir := filepath.Join(e.home, "regclean-backup", "2026-11-26_12-00-00")
	if _, err := os.Stat(wantDir); err != nil {
		t.Errorf("default backup dir missing: %v", err)
	}
}

func TestDeleteWithoutAdminSkipsMachine(t *testing.T) {
	e := newEnv(t, false)
	code, out, _ := e.run(t, "", "-delete", "-yes")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if !e.store.HasValue(hklm64, runKey, "DeadMachine") {
		t.Error("HKLM entry removed without admin rights")
	}
	if e.store.HasValue(hkcu, runKey, "DeadUser") {
		t.Error("HKCU entry should still be removed")
	}
	if !strings.Contains(out, "Administrator") {
		t.Errorf("no admin hint:\n%s", out)
	}
}

func TestInteractive(t *testing.T) {
	e := newEnv(t, true)
	// entries are asked in scan order: HKCU first, then HKLM64
	code, out, _ := e.run(t, "n\nj\n", "-delete")
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if !e.store.HasValue(hkcu, runKey, "DeadUser") {
		t.Error("answered n but entry was removed")
	}
	if e.store.HasValue(hklm64, runKey, "DeadMachine") {
		t.Error("answered j but entry is still there")
	}
}

func TestEndOfInputAborts(t *testing.T) {
	e := newEnv(t, true)
	code, out, _ := e.run(t, "", "-delete")
	if code != exitOK || !strings.Contains(out, "Abgebrochen") {
		t.Errorf("exit %d\n%s", code, out)
	}
	if !e.store.HasValue(hkcu, runKey, "DeadUser") {
		t.Error("EOF on stdin must not delete anything")
	}
}

func TestBackupFailureIsReported(t *testing.T) {
	e := newEnv(t, true)
	e.store.ExportErr = os.ErrPermission
	code, out, _ := e.run(t, "", "-delete", "-yes")
	if code != exitFailure || !strings.Contains(out, "FEHLER") {
		t.Errorf("exit %d\n%s", code, out)
	}
	if !e.store.HasValue(hkcu, runKey, "DeadUser") {
		t.Error("removed without backup")
	}
}

func TestUsageErrors(t *testing.T) {
	e := newEnv(t, true)
	if code, _, errOut := e.run(t, "", "-yes"); code != exitUsage || !strings.Contains(errOut, "-delete") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
	if code, _, _ := e.run(t, "", "-h"); code != exitOK {
		t.Errorf("-h exit %d", code)
	}
}

func TestInterrupted(t *testing.T) {
	e := newEnv(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	if code := run(ctx, nil, e.deps, strings.NewReader(""), &out, &errOut); code != exitInterrupted {
		t.Errorf("exit %d", code)
	}
}
