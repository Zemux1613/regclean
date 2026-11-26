package scan

import (
	"regclean/internal/pathcheck"
	"regclean/internal/reg"
)

const currentVersion = `Software\Microsoft\Windows\CurrentVersion`

// Location is a registry key below CurrentVersion that can hold orphaned entries.
type Location struct {
	Label  string
	SubKey string
	Kind   Kind
	eval   evaluator // only for KindKey
}

type evaluator func(k reg.Key, c *pathcheck.Checker) pathcheck.Result

// areas maps the names accepted by -areas to their locations.
var areas = []struct {
	name string
	locs []Location
}{
	{"run", []Location{
		{Label: "Run", SubKey: "Run", Kind: KindValue},
		{Label: "RunOnce", SubKey: "RunOnce", Kind: KindValue},
	}},
	{"uninstall", []Location{
		{Label: "Uninstall", SubKey: "Uninstall", Kind: KindKey, eval: evalUninstall},
	}},
	{"apppaths", []Location{
		{Label: "AppPaths", SubKey: "App Paths", Kind: KindKey, eval: evalAppPath},
	}},
}

// AreaNames lists the valid area names in display order.
func AreaNames() []string {
	names := make([]string, len(areas))
	for i, a := range areas {
		names[i] = a.name
	}
	return names
}

// LocationsFor returns the locations of the named areas. Unknown names are ignored.
func LocationsFor(names []string) []Location {
	var out []Location
	for _, n := range names {
		for _, a := range areas {
			if a.name == n {
				out = append(out, a.locs...)
			}
		}
	}
	return out
}

// evalUninstall is deliberately conservative: an entry only counts as orphaned
// if the uninstaller, the install directory and the icon are all gone.
func evalUninstall(k reg.Key, c *pathcheck.Checker) pathcheck.Result {
	if v, ok := k.DWord("WindowsInstaller"); ok && v == 1 {
		return undecided("MSI-Eintrag")
	}
	if str(k, "ParentKeyName") != "" {
		return undecided("Update/Patch-Eintrag")
	}
	cmd := str(k, "UninstallString")
	if cmd == "" {
		cmd = str(k, "QuietUninstallString")
	}
	if cmd == "" {
		return undecided("kein UninstallString")
	}

	res := c.Command(cmd)
	if res.Status != pathcheck.Orphaned {
		return res
	}
	// The uninstaller is gone; only call the entry orphaned if nothing else is left.
	if loc := str(k, "InstallLocation"); loc != "" && c.Dir(loc).Status != pathcheck.Orphaned {
		return survivor(res, "InstallLocation")
	}
	if icon := pathcheck.IconFile(str(k, "DisplayIcon")); icon != "" && c.Command(icon).Status != pathcheck.Orphaned {
		return survivor(res, "DisplayIcon")
	}
	return res
}

func evalAppPath(k reg.Key, c *pathcheck.Checker) pathcheck.Result {
	def := str(k, "")
	if def == "" {
		return undecided("no default value")
	}
	return c.Command(def)
}

func survivor(res pathcheck.Result, what string) pathcheck.Result {
	return pathcheck.Result{
		Status: pathcheck.Unknown,
		Target: res.Target,
		Note:   "Uninstaller is missing, " + what + " but still there/cannot be verified",
	}
}

func undecided(note string) pathcheck.Result {
	return pathcheck.Result{Status: pathcheck.Unknown, Note: note}
}

// str returns a string value or "" if missing.
func str(k reg.Key, name string) string {
	v, _ := k.String(name)
	return v
}
