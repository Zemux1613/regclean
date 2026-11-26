package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"regclean/internal/reg"
	"regclean/internal/scan"
)

type scope int

const (
	scopeAll scope = iota
	scopeUser
	scopeMachine
)

type config struct {
	areas     []string
	scope     scope
	delete    bool
	yes       bool
	verbose   bool
	backupDir string
}

// hives returns the hives selected by the scope.
func (c config) hives() []reg.Hive {
	var out []reg.Hive
	for _, h := range reg.Hives() {
		switch {
		case c.scope == scopeUser && h.NeedsAdmin():
		case c.scope == scopeMachine && !h.NeedsAdmin():
		default:
			out = append(out, h)
		}
	}
	return out
}

// parseFlags parses args. For -h it prints the usage and returns flag.ErrHelp.
func parseFlags(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("regclean", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {} // we print usage ourselves, once, and only for -h

	var (
		areas   = fs.String("areas", strings.Join(scan.AreaNames(), ","), "Areas to be reviewed, separated by commas")
		scopeS  = fs.String("scope", "all", "user (HKCU), machine (HKLM) or all")
		del     = fs.Bool("delete", false, "Actually delete orphaned entries (default: display only)")
		yes     = fs.Bool("yes", false, "Do not prompt individually when deleting (only with -delete)")
		verbose = fs.Bool("v", false, "Also list entries that are intact and cannot be verified")
		backup  = fs.String("backup", "", `Folder for .reg backups (Default: <User Folder>\regclean-backup\<Timestamp>)`)
	)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stderr, fs)
		}
		return config{}, err
	}
	if fs.NArg() > 0 {
		return config{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *yes && !*del {
		return config{}, errors.New("-yes only makes sense when used with -delete")
	}

	cfg := config{delete: *del, yes: *yes, verbose: *verbose, backupDir: *backup}
	var err error
	if cfg.areas, err = parseAreas(*areas); err != nil {
		return config{}, err
	}
	if cfg.scope, err = parseScope(*scopeS); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func parseAreas(csv string) ([]string, error) {
	valid := map[string]bool{}
	for _, n := range scan.AreaNames() {
		valid[n] = true
	}
	var out []string
	seen := map[string]bool{}
	for a := range strings.SplitSeq(strings.ToLower(csv), ",") {
		a = strings.TrimSpace(a)
		switch {
		case a == "" || seen[a]:
		case !valid[a]:
			return nil, fmt.Errorf("unknown area %q (valid: %s)", a, strings.Join(scan.AreaNames(), ", "))
		default:
			seen[a] = true
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("-areas is empty")
	}
	return out, nil
}

func parseScope(s string) (scope, error) {
	switch s {
	case "all":
		return scopeAll, nil
	case "user":
		return scopeUser, nil
	case "machine":
		return scopeMachine, nil
	}
	return 0, fmt.Errorf("-scope must be user, machine, or all; it cannot be %q", s)
}

func printUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, "regclean - Find and clean up orphaned registry entries")
	fmt.Fprintln(w, "\nWithout the -delete option, it's a dry run: nothing is changed.")
	fmt.Fprintln(w, "\nOptions:")
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
}
