package scan_test

import (
	"context"
	"testing"

	"regclean/internal/pathcheck"
	"regclean/internal/pathcheck/pathchecktest"
	"regclean/internal/reg"
	"regclean/internal/reg/regtest"
	"regclean/internal/scan"
)

const cv = `Software\Microsoft\Windows\CurrentVersion`

var hkcu = reg.Hive{Root: reg.HKCU}

func fixture() (*regtest.Store, *pathchecktest.FS) {
	fs := pathchecktest.New(`C:\Apps\good.exe`)
	fs.Drives["D:"] = false

	s := regtest.New()
	// Run
	s.Set(hkcu, cv+`\Run`, "Good", `"C:\Apps\good.exe" -x`)
	s.Set(hkcu, cv+`\Run`, "Dead", `"C:\Apps\dead.exe"`)
	s.Set(hkcu, cv+`\Run`, "Offline", `D:\x\y.exe`)
	s.Set(hkcu, cv+`\Run`, "Binary", 5) // not a string value: ignored

	// Uninstall
	u := cv + `\Uninstall\`
	s.Set(hkcu, u+"GoneApp", "DisplayName", "Gone App")
	s.Set(hkcu, u+"GoneApp", "UninstallString", `"C:\Gone\unins000.exe"`)
	s.Set(hkcu, u+"InstallDirAlive", "UninstallString", `"C:\Gone\unins000.exe"`)
	s.Set(hkcu, u+"InstallDirAlive", "InstallLocation", `C:\Apps`)
	s.Set(hkcu, u+"IconAlive", "UninstallString", `"C:\Gone\unins000.exe"`)
	s.Set(hkcu, u+"IconAlive", "DisplayIcon", `C:\Apps\good.exe,0`)
	s.Set(hkcu, u+"Msi", "WindowsInstaller", 1)
	s.Set(hkcu, u+"Msi", "UninstallString", `"C:\Gone\x.exe"`)
	s.Set(hkcu, u+"NoInfo", "DisplayName", "Nothing to go on")
	s.Set(hkcu, u+"Patch", "ParentKeyName", "Parent")
	s.Set(hkcu, u+"Patch", "UninstallString", `"C:\Gone\x.exe"`)

	// App Paths
	a := cv + `\App Paths\`
	s.Set(hkcu, a+"dead.exe", "", `C:\Apps\dead.exe`)
	s.Set(hkcu, a+"good.exe", "", `C:\Apps\good.exe`)
	return s, fs
}

func TestScan(t *testing.T) {
	s, fs := fixture()
	sc := scan.New(s, pathcheck.New(fs))
	got, err := sc.Scan(context.Background(), []reg.Hive{hkcu}, scan.LocationsFor(scan.AreaNames()))
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]pathcheck.Status{
		"Good":                      pathcheck.OK,
		"Dead":                      pathcheck.Orphaned,
		"Offline":                   pathcheck.Unknown,
		"GoneApp (Gone App)":        pathcheck.Orphaned,
		"InstallDirAlive":           pathcheck.Unknown,
		"IconAlive":                 pathcheck.Unknown,
		"Msi":                       pathcheck.Unknown,
		"NoInfo (Nothing to go on)": pathcheck.Unknown,
		"Patch":                     pathcheck.Unknown,
		"dead.exe":                  pathcheck.Orphaned,
		"good.exe":                  pathcheck.OK,
	}
	seen := map[string]bool{}
	for _, e := range got {
		w, ok := want[e.Title]
		if !ok {
			t.Errorf("unexpected entry %q", e.Title)
			continue
		}
		seen[e.Title] = true
		if e.Verdict.Status != w {
			t.Errorf("%s: status %v (%s), want %v", e.Title, e.Verdict.Status, e.Verdict.Note, w)
		}
	}
	for title := range want {
		if !seen[title] {
			t.Errorf("missing entry %q", title)
		}
	}

	sum := scan.Summarize(got)
	if sum.Orphaned != 3 || sum.Checked != len(want) {
		t.Errorf("summary = %+v", sum)
	}
	if n := len(scan.Orphans(got)); n != 3 {
		t.Errorf("Orphans = %d, want 3", n)
	}
}

func TestScanMissingKeysAreFine(t *testing.T) {
	sc := scan.New(regtest.New(), pathcheck.New(pathchecktest.New()))
	got, err := sc.Scan(context.Background(), reg.Hives(), scan.LocationsFor(scan.AreaNames()))
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestScanCancelled(t *testing.T) {
	s, fs := fixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := scan.New(s, pathcheck.New(fs)).Scan(ctx, []reg.Hive{hkcu}, scan.LocationsFor(scan.AreaNames()))
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestHivesAreSeparate(t *testing.T) {
	fs := pathchecktest.New()
	s := regtest.New()
	h32 := reg.Hive{Root: reg.HKLM, View: reg.View32}
	s.Set(h32, cv+`\Run`, "Dead32", `C:\gone.exe`)

	sc := scan.New(s, pathcheck.New(fs))
	got, _ := sc.Scan(context.Background(), []reg.Hive{{Root: reg.HKLM, View: reg.View64}}, scan.LocationsFor([]string{"run"}))
	if len(got) != 0 {
		t.Errorf("64-bit view must not see the 32-bit key: %v", got)
	}
	got, _ = sc.Scan(context.Background(), []reg.Hive{h32}, scan.LocationsFor([]string{"run"}))
	if len(got) != 1 || !got[0].Orphaned() {
		t.Errorf("32-bit view: %v", got)
	}
}
