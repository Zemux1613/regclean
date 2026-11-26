// Package pathcheck decides whether the file a registry value points to still
// exists. It is conservative: anything it cannot judge with certainty is
// reported as Unknown, and callers must never touch Unknown entries.
package pathcheck

import "strings"

// Status is the verdict for one command line or path.
type Status int

const (
	OK       Status = iota // target exists
	Orphaned               // target is definitely gone
	Unknown                // cannot be decided (offline drive, interpreter, ...)
)

func (s Status) String() string {
	switch s {
	case OK:
		return "ok"
	case Orphaned:
		return "orphaned"
	default:
		panic("unhandled default case")
	}
	return "unknown"
}

// Result of a check.
type Result struct {
	Status Status
	Target string // the file or directory that was looked at
	Note   string // reason, mostly for Unknown
}

// FS is the file system as seen by the checker.
type FS interface {
	Exists(path string) bool       // file or directory
	IsFile(path string) bool       // regular file
	DriveOnline(drive string) bool // drive like "C:"
	LookupBare(name string) bool   // bare name like "ctfmon.exe" resolves via PATH
}

// Checker evaluates command lines against an FS.
type Checker struct{ fs FS }

// New returns a Checker using fs.
func New(fs FS) *Checker { return &Checker{fs: fs} }

// Programs whose arguments are not interpreted.
var opaqueHosts = map[string]bool{
	"cmd.exe": true, "cmd": true, "powershell.exe": true, "pwsh.exe": true,
	"wscript.exe": true, "cscript.exe": true, "mshta.exe": true,
	"msiexec.exe": true, "msiexec": true, "regsvr32.exe": true,
	"explorer.exe": true, "control.exe": true,
}

var knownExts = []string{".exe", ".com", ".bat", ".cmd", ".lnk", ".msc", ".cpl", ".dll", ".scr"}

// Command checks the executable of a command line as stored in Run,
// Uninstall or App Paths values. %VARIABLES% must already be expanded.
func (c *Checker) Command(cmd string) Result {
	cmd = strings.TrimPrefix(strings.TrimSpace(cmd), `\??\`)
	if cmd == "" {
		return unknown("", "leer")
	}
	exe, args := c.splitCommand(cmd)

	switch base := strings.ToLower(baseName(exe)); {
	case base == "rundll32.exe" || base == "rundll32":
		dll := rundllTarget(args)
		if dll == "" {
			return unknown(exe, "rundll32 ohne erkennbare DLL")
		}
		return c.checkPath(dll, false)
	case opaqueHosts[base]:
		return unknown(exe, "Interpreter/Host - Argumente werden nicht geprüft")
	}
	return c.checkPath(exe, false)
}

// Dir checks a plain directory path such as InstallLocation.
func (c *Checker) Dir(p string) Result {
	p = strings.Trim(strings.TrimSpace(p), `"`)
	if p == "" {
		return unknown("", "empty")
	}
	return c.checkPath(p, true)
}

func (c *Checker) checkPath(p string, wantDir bool) Result {
	switch {
	case strings.HasPrefix(p, `\\`):
		return unknown(p, "Network path")
	case hasDrive(p):
		return c.onDrive(p, wantDir)
	case !strings.ContainsAny(p, `\/`):
		return c.bare(p, wantDir)
	default:
		return unknown(p, "relative path")
	}
}

func (c *Checker) onDrive(p string, wantDir bool) Result {
	drive := strings.ToUpper(p[:2])
	if !c.fs.DriveOnline(drive) {
		return unknown(p, "Drive "+drive+" not available")
	}
	found := false
	if wantDir {
		found = c.fs.Exists(p)
	} else {
		found = c.fileWithExt(p)
	}
	if found {
		return Result{Status: OK, Target: p}
	}
	return Result{Status: Orphaned, Target: p}
}

func (c *Checker) bare(name string, wantDir bool) Result {
	if !wantDir && (c.fs.LookupBare(name) || c.fs.LookupBare(name+".exe")) {
		return Result{Status: OK, Target: name}
	}
	return unknown(name, "Name cannot be resolved without a path (possibly app paths)")
}

// fileWithExt mimics CreateProcess: try the name as is, then with ".exe".
func (c *Checker) fileWithExt(p string) bool {
	if c.fs.IsFile(p) {
		return true
	}
	return !hasKnownExt(p) && c.fs.IsFile(p+".exe")
}

// splitCommand separates executable and arguments the way CreateProcess does:
// quoted -> up to the closing quote; unquoted -> shortest existing prefix at a
// space; otherwise a guess based on known extensions.
func (c *Checker) splitCommand(cmd string) (exe, args string) {
	if cmd[0] == '"' {
		end := strings.IndexByte(cmd[1:], '"')
		if end == -1 {
			return cmd[1:], ""
		}
		return cmd[1 : 1+end], strings.TrimSpace(cmd[end+2:])
	}

	cands := spacePrefixes(cmd)
	for _, p := range cands {
		if len(p) >= 3 && hasDrive(p) && c.fileWithExt(p) {
			return p, tail(cmd, p)
		}
	}
	for _, p := range cands {
		if hasKnownExt(p) {
			return p, tail(cmd, p)
		}
	}
	if before, _, ok := strings.Cut(cmd, " "); ok {
		return before, tail(cmd, before)
	}
	return cmd, ""
}

// IconFile extracts the file from a DisplayIcon value like `"C:\x\a.exe",0`.
func IconFile(v string) string {
	v = strings.TrimSpace(v)
	switch {
	case v == "":
		return ""
	case v[0] == '"':
		if end := strings.IndexByte(v[1:], '"'); end != -1 {
			return v[1 : 1+end]
		}
		return strings.Trim(v, `"`)
	}
	if i := strings.LastIndexByte(v, ','); i != -1 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

func rundllTarget(args string) string {
	args = strings.TrimSpace(args)
	switch {
	case args == "":
		return ""
	case args[0] == '"':
		if end := strings.IndexByte(args[1:], '"'); end != -1 {
			return args[1 : 1+end]
		}
		return ""
	}
	if i := strings.IndexAny(args, ", "); i != -1 {
		return strings.TrimSpace(args[:i])
	}
	return args
}

// spacePrefixes returns cmd cut at every space, plus cmd itself.
func spacePrefixes(cmd string) []string {
	var out []string
	for i := 0; i < len(cmd); i++ {
		if cmd[i] == ' ' {
			out = append(out, cmd[:i])
		}
	}
	return append(out, cmd)
}

func tail(cmd, exe string) string { return strings.TrimSpace(cmd[len(exe):]) }

func hasKnownExt(p string) bool {
	l := strings.ToLower(p)
	for _, e := range knownExts {
		if strings.HasSuffix(l, e) {
			return true
		}
	}
	return false
}

func hasDrive(p string) bool {
	if len(p) < 2 || p[1] != ':' {
		return false
	}
	c := p[0] | 0x20
	return c >= 'a' && c <= 'z'
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `\/`); i != -1 {
		return p[i+1:]
	}
	return p
}

func unknown(target, note string) Result {
	return Result{Status: Unknown, Target: target, Note: note}
}
