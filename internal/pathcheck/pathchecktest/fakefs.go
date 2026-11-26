// Package pathchecktest provides an in-memory pathcheck.FS for tests.
package pathchecktest

import "strings"

// FS is an in-memory file system. Drive C: is online by default.
type FS struct {
	files map[string]bool
	dirs  map[string]bool

	Drives map[string]bool // "D:" -> online
	Bare   map[string]bool // names LookupBare resolves, lower-case
}

// New returns an FS containing the given files (parent dirs are implied).
func New(files ...string) *FS {
	f := &FS{
		files:  map[string]bool{},
		dirs:   map[string]bool{},
		Drives: map[string]bool{"C:": true},
		Bare:   map[string]bool{},
	}
	for _, p := range files {
		f.AddFile(p)
	}
	return f
}

func norm(p string) string { return strings.ToLower(strings.TrimRight(p, `\`)) }

func (f *FS) AddFile(p string) { f.files[norm(p)] = true; f.addParents(p) }
func (f *FS) AddDir(p string)  { f.dirs[norm(p)] = true; f.addParents(p) }

func (f *FS) addParents(p string) {
	p = norm(p)
	for {
		i := strings.LastIndex(p, `\`)
		if i <= 2 {
			return
		}
		p = p[:i]
		f.dirs[p] = true
	}
}

func (f *FS) Exists(p string) bool { return f.files[norm(p)] || f.dirs[norm(p)] }
func (f *FS) IsFile(p string) bool { return f.files[norm(p)] }

func (f *FS) DriveOnline(d string) bool { return f.Drives[strings.ToUpper(d)] }
func (f *FS) LookupBare(n string) bool  { return f.Bare[strings.ToLower(n)] }
