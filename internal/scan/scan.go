// Package scan walks registry locations and decides which entries are orphaned.
// It performs no I/O besides reading through reg.Store and pathcheck.FS.
package scan

import (
	"context"
	"errors"
	"fmt"

	"regclean/internal/pathcheck"
	"regclean/internal/reg"
)

// Kind tells what an Entry is.
type Kind int

const (
	KindValue Kind = iota // one value inside a key, e.g. in Run
	KindKey               // a whole subkey, e.g. Uninstall\Foo
)

// Entry is one examined registry entry together with its verdict.
type Entry struct {
	Area    string // e.g. "Run", "Uninstall"
	Hive    reg.Hive
	Key     string // KindValue: key holding the value; KindKey: the key itself
	Value   string // value name; empty for KindKey
	Kind    Kind
	Title   string // for humans
	Verdict pathcheck.Result
}

// Orphaned reports whether the entry points to something that is gone.
func (e Entry) Orphaned() bool { return e.Verdict.Status == pathcheck.Orphaned }

// Scanner examines registry locations.
type Scanner struct {
	store   reg.Store
	checker *pathcheck.Checker
}

// New returns a Scanner.
func New(store reg.Store, checker *pathcheck.Checker) *Scanner {
	return &Scanner{store: store, checker: checker}
}

// Scan examines every location in every hive. Keys that do not exist are
// skipped silently; other read errors are collected and returned together with
// the partial result, so one unreadable key does not hide the rest.
func (s *Scanner) Scan(ctx context.Context, hives []reg.Hive, locs []Location) ([]Entry, error) {
	var (
		entries []Entry
		errs    []error
	)
	for _, h := range hives {
		for _, loc := range locs {
			if err := ctx.Err(); err != nil {
				return entries, err
			}
			got, err := s.scanLocation(h, loc)
			entries = append(entries, got...)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return entries, errors.Join(errs...)
}

func (s *Scanner) scanLocation(h reg.Hive, loc Location) ([]Entry, error) {
	path := currentVersion + `\` + loc.SubKey
	key, err := s.store.Open(h, path)
	if errors.Is(err, reg.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer key.Close()

	if loc.Kind == KindValue {
		return s.scanValues(h, loc, path, key)
	}
	return s.scanSubKeys(h, loc, path, key)
}

func (s *Scanner) scanValues(h reg.Hive, loc Location, path string, key reg.Key) ([]Entry, error) {
	names, err := key.ValueNames()
	if err != nil {
		return nil, fmt.Errorf("%s\\%s: %w", h, path, err)
	}
	var entries []Entry
	for _, name := range names {
		if name == "" {
			continue // default value
		}
		cmd, ok := key.String(name)
		if !ok {
			continue // not a string value
		}
		entries = append(entries, Entry{
			Area: loc.Label, Hive: h, Key: path, Value: name, Kind: KindValue,
			Title: name, Verdict: s.checker.Command(cmd),
		})
	}
	return entries, nil
}

func (s *Scanner) scanSubKeys(h reg.Hive, loc Location, path string, key reg.Key) ([]Entry, error) {
	names, err := key.SubKeyNames()
	if err != nil {
		return nil, fmt.Errorf("%s\\%s: %w", h, path, err)
	}
	var (
		entries []Entry
		errs    []error
	)
	for _, name := range names {
		subPath := path + `\` + name
		sub, err := s.store.Open(h, subPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		title := name
		if dn := str(sub, "DisplayName"); dn != "" {
			title = name + " (" + dn + ")"
		}
		verdict := loc.eval(sub, s.checker)
		sub.Close()

		entries = append(entries, Entry{
			Area: loc.Label, Hive: h, Key: subPath, Kind: KindKey,
			Title: title, Verdict: verdict,
		})
	}
	return entries, errors.Join(errs...)
}

// Orphans returns only the orphaned entries.
func Orphans(entries []Entry) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.Orphaned() {
			out = append(out, e)
		}
	}
	return out
}

// Summary counts the verdicts of a scan.
type Summary struct {
	Checked, Unknown, Orphaned int
}

// Summarize counts the verdicts in entries.
func Summarize(entries []Entry) Summary {
	s := Summary{Checked: len(entries)}
	for _, e := range entries {
		switch e.Verdict.Status {
		case pathcheck.Unknown:
			s.Unknown++
		case pathcheck.Orphaned:
			s.Orphaned++
		default:
			panic("unhandled default case")
		}
	}
	return s
}
