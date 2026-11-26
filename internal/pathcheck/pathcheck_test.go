package pathcheck_test

import (
	"testing"

	"regclean/internal/pathcheck"
	"regclean/internal/pathcheck/pathchecktest"
)

func newFS() *pathchecktest.FS {
	fs := pathchecktest.New(
		`C:\Program Files\Foo Bar\foo.exe`,
		`C:\Users\x\AppData\Local\Microsoft\Teams\Update.exe`,
		`C:\Windows\System32\shell32.dll`,
		`C:\Tools\Foo bar.exe`,
	)
	fs.AddDir(`C:\Tools\Foo`) // a directory must not win over the file "Foo bar.exe"
	fs.Bare["ctfmon.exe"] = true
	return fs
}

func TestCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want pathcheck.Status
	}{
		{"quoted existing", `"C:\Program Files\Foo Bar\foo.exe" -silent`, pathcheck.OK},
		{"quoted missing", `"C:\Program Files\Foo Bar\gone.exe" -silent`, pathcheck.Orphaned},
		{"unquoted with spaces, existing", `C:\Program Files\Foo Bar\foo.exe -x`, pathcheck.OK},
		{"unquoted with spaces, missing", `C:\Program Files\Foo Bar\gone.exe -x`, pathcheck.Orphaned},
		{"teams stub still exists", `C:\Users\x\AppData\Local\Microsoft\Teams\Update.exe --uninstall`, pathcheck.OK},
		{"dir prefix must not win", `C:\Tools\Foo bar.exe /s`, pathcheck.OK},
		{"implicit .exe", `C:\Program Files\Foo Bar\foo`, pathcheck.OK},
		{"offline drive", `D:\Programme\Nextcloud\nextcloud.exe`, pathcheck.Unknown},
		{"unc path", `\\server\share\x.exe`, pathcheck.Unknown},
		{"bare name found", `ctfmon.exe`, pathcheck.OK},
		{"bare name unknown", `mystery.exe`, pathcheck.Unknown},
		{"interpreter", `cmd.exe /c C:\gone.bat`, pathcheck.Unknown},
		{"rundll32 existing", `rundll32.exe C:\Windows\System32\shell32.dll,Foo`, pathcheck.OK},
		{"rundll32 missing", `rundll32.exe "C:\Windows\System32\gone.dll",Foo`, pathcheck.Orphaned},
		{"relative path", `..\foo.exe`, pathcheck.Unknown},
		{"empty", ``, pathcheck.Unknown},
		{"nt prefix", `\??\C:\Program Files\Foo Bar\foo.exe`, pathcheck.OK},
	}
	c := pathcheck.New(newFS())
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.Command(tc.cmd)
			if got.Status != tc.want {
				t.Errorf("Command(%q) = %v (target %q, note %q), want %v",
					tc.cmd, got.Status, got.Target, got.Note, tc.want)
			}
		})
	}
}

func TestDir(t *testing.T) {
	c := pathcheck.New(newFS())
	if r := c.Dir(`C:\Program Files\Foo Bar\`); r.Status != pathcheck.OK {
		t.Errorf("existing dir with trailing backslash: %+v", r)
	}
	if r := c.Dir(`C:\Gone`); r.Status != pathcheck.Orphaned {
		t.Errorf("missing dir: %+v", r)
	}
	if r := c.Dir(`D:\Anything`); r.Status != pathcheck.Unknown {
		t.Errorf("offline drive: %+v", r)
	}
}

func TestIconFile(t *testing.T) {
	for in, want := range map[string]string{
		`"C:\a b\x.exe",0`: `C:\a b\x.exe`,
		`C:\a\x.exe,1`:     `C:\a\x.exe`,
		`C:\a\x.ico`:       `C:\a\x.ico`,
		``:                 ``,
	} {
		if got := pathcheck.IconFile(in); got != want {
			t.Errorf("IconFile(%q) = %q, want %q", in, got, want)
		}
	}
}
