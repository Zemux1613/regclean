package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestParseFlagsDefaults(t *testing.T) {
	cfg, err := parseFlags(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cfg.areas, ",") != "run,uninstall,apppaths" || cfg.delete || cfg.scope != scopeAll {
		t.Errorf("cfg = %+v", cfg)
	}
	if len(cfg.hives()) != 3 {
		t.Errorf("hives = %v", cfg.hives())
	}
}

func TestParseFlagsScope(t *testing.T) {
	cfg, _ := parseFlags([]string{"-scope", "user"}, &bytes.Buffer{})
	if h := cfg.hives(); len(h) != 1 || h[0].NeedsAdmin() {
		t.Errorf("user scope hives = %v", h)
	}
	cfg, _ = parseFlags([]string{"-scope", "machine"}, &bytes.Buffer{})
	if h := cfg.hives(); len(h) != 2 || !h[0].NeedsAdmin() {
		t.Errorf("machine scope hives = %v", h)
	}
}

func TestParseFlagsErrors(t *testing.T) {
	for _, args := range [][]string{
		{"-areas", "foo"},
		{"-areas", ","},
		{"-scope", "nope"},
		{"-yes"},
		{"-unknown"},
		{"stray"},
	} {
		if _, err := parseFlags(args, &bytes.Buffer{}); err == nil {
			t.Errorf("parseFlags(%v) should fail", args)
		}
	}
}

func TestParseFlagsHelp(t *testing.T) {
	var buf bytes.Buffer
	_, err := parseFlags([]string{"-h"}, &buf)
	if !errors.Is(err, flag.ErrHelp) || !strings.Contains(buf.String(), "-delete") {
		t.Errorf("err = %v, usage = %q", err, buf.String())
	}
}

func TestParseAreasDedupe(t *testing.T) {
	got, err := parseAreas("Run, run,uninstall")
	if err != nil || strings.Join(got, ",") != "run,uninstall" {
		t.Errorf("got %v, %v", got, err)
	}
}
