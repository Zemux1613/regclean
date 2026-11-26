package reg

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// IsElevated reports whether the process runs with administrator rights.
func IsElevated() bool { return windows.GetCurrentProcessToken().IsElevated() }

// NewStore returns a Store backed by the real Windows registry.
//
// Reading and value deletion use the registry API. Export and key deletion go
// through reg.exe: it handles the 32/64-bit views and recursion reliably,
// which the API's DeleteKey does not.
func NewStore() Store { return winStore{} }

type winStore struct{}

func (winStore) Open(h Hive, path string) (Key, error) {
	access := uint32(registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS) | viewAccess(h.View)
	k, err := registry.OpenKey(rootKey(h.Root), path, access)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil, fmt.Errorf("%s\\%s: %w", h, path, ErrNotExist)
		}
		return nil, fmt.Errorf("open %s\\%s: %w", h, path, err)
	}
	return winKey{k}, nil
}

func (winStore) DeleteValue(h Hive, path, name string) error {
	k, err := registry.OpenKey(rootKey(h.Root), path, registry.SET_VALUE|viewAccess(h.View))
	if err != nil {
		return fmt.Errorf("open %s\\%s: %w", h, path, err)
	}
	defer k.Close()
	if err := k.DeleteValue(name); err != nil {
		return fmt.Errorf("delete value %q in %s\\%s: %w", name, h, path, err)
	}
	return nil
}

func (winStore) DeleteKey(h Hive, path string) error {
	return regExe(h, "delete", path, "/f")
}

func (winStore) Export(h Hive, path, file string) error {
	return regExe(h, "export", path, file, "/y")
}

func regExe(h Hive, verb, path string, extra ...string) error {
	args := append([]string{verb, string(h.Root) + `\` + path}, extra...)
	switch h.View {
	case View32:
		args = append(args, "/reg:32")
	case View64:
		args = append(args, "/reg:64")
	default:
		panic("unhandled default case")
	}
	out, err := exec.Command("reg", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("reg %s %s\\%s: %w: %s", verb, h, path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func rootKey(r Root) registry.Key {
	if r == HKLM {
		return registry.LOCAL_MACHINE
	}
	return registry.CURRENT_USER
}

func viewAccess(v View) uint32 {
	switch v {
	case View64:
		return registry.WOW64_64KEY
	case View32:
		return registry.WOW64_32KEY
	default:
		panic("unhandled default case")
	}
	return 0
}

type winKey struct{ k registry.Key }

func (w winKey) String(name string) (string, bool) {
	v, _, err := w.k.GetStringValue(name)
	if err != nil {
		return "", false
	}
	if strings.Contains(v, "%") {
		if e, err := registry.ExpandString(v); err == nil {
			return e, true
		}
	}
	return v, true
}

func (w winKey) DWord(name string) (uint64, bool) {
	v, _, err := w.k.GetIntegerValue(name)
	return v, err == nil
}

func (w winKey) ValueNames() ([]string, error)  { return w.k.ReadValueNames(-1) }
func (w winKey) SubKeyNames() ([]string, error) { return w.k.ReadSubKeyNames(-1) }
func (w winKey) Close() error                   { return w.k.Close() }
