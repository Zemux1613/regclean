// Package reg abstracts the few registry operations regclean needs, so the
// scanning and cleaning logic can be tested without a real registry.
package reg

import "io/fs"

// ErrNotExist is wrapped by Store.Open when a key does not exist.
var ErrNotExist = fs.ErrNotExist

// Root is a registry root.
type Root string

const (
	HKCU Root = "HKCU"
	HKLM Root = "HKLM"
)

// View selects the WOW64 registry view.
type View int

const (
	ViewDefault View = iota // native view of the caller (only meaningful for HKCU)
	View64
	View32
)

// Hive is a root together with a registry view.
type Hive struct {
	Root Root
	View View
}

// String returns HKCU, HKLM64 or HKLM32.
func (h Hive) String() string {
	switch h.View {
	case View64:
		return string(h.Root) + "64"
	case View32:
		return string(h.Root) + "32"
	default:
		panic("unhandled default case")
	}
	return string(h.Root)
}

// NeedsAdmin reports whether modifying this hive requires elevation.
func (h Hive) NeedsAdmin() bool { return h.Root == HKLM }

// Hives returns every hive regclean knows about.
func Hives() []Hive {
	return []Hive{
		{Root: HKCU, View: ViewDefault},
		{Root: HKLM, View: View64},
		{Root: HKLM, View: View32},
	}
}

// Key is an open, read-only registry key.
type Key interface {
	// String returns a REG_SZ/REG_EXPAND_SZ value with %VARIABLES% expanded.
	// The empty name addresses the default value.
	String(name string) (value string, ok bool)
	// DWord returns a REG_DWORD/REG_QWORD value.
	DWord(name string) (value uint64, ok bool)
	ValueNames() ([]string, error)
	SubKeyNames() ([]string, error)
	Close() error
}

// Store gives access to the registry. Paths are relative to the hive root
// and use backslashes.
type Store interface {
	// Open opens a key read-only. A missing key yields an error wrapping ErrNotExist.
	Open(h Hive, path string) (Key, error)
	DeleteValue(h Hive, path, name string) error
	// DeleteKey removes the key including all subkeys.
	DeleteKey(h Hive, path string) error
	// Export writes the key (recursively) as .reg file; nil means the file exists.
	Export(h Hive, path, file string) error
}
