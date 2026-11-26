package pathcheck

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NewOSFS returns an FS backed by the real file system.
//
// Errors other than "does not exist" (access denied, invalid name, device not
// ready, ...) count as "exists": regclean only flags what it is sure about.
func NewOSFS() FS {
	winDir := os.Getenv("SystemRoot")
	o := osFS{}
	if winDir != "" {
		o.sys32 = strings.ToLower(filepath.Join(winDir, "System32")) + `\`
		o.wow64 = filepath.Join(winDir, "SysWOW64")
	}
	return o
}

type osFS struct {
	sys32 string // lower-case "C:\windows\system32\"
	wow64 string // "C:\Windows\SysWOW64"
}

// variants adds the SysWOW64 twin for System32 paths: 32-bit entries see
// SysWOW64 where this (64-bit) process sees System32.
func (o osFS) variants(p string) []string {
	if o.sys32 != "" && strings.HasPrefix(strings.ToLower(p), o.sys32) {
		return []string{p, filepath.Join(o.wow64, p[len(o.sys32):])}
	}
	return []string{p}
}

func probe(p string) (found, isDir bool) {
	fi, err := os.Stat(p)
	if err == nil {
		return true, fi.IsDir()
	}
	return !errors.Is(err, fs.ErrNotExist), false
}

func (o osFS) Exists(p string) bool {
	for _, v := range o.variants(p) {
		if found, _ := probe(v); found {
			return true
		}
	}
	return false
}

func (o osFS) IsFile(p string) bool {
	for _, v := range o.variants(p) {
		if found, dir := probe(v); found && !dir {
			return true
		}
	}
	return false
}

func (osFS) DriveOnline(drive string) bool {
	_, err := os.Stat(drive + `\`)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}

func (osFS) LookupBare(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil || errors.Is(err, exec.ErrDot)
}
