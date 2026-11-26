// Package regtest provides an in-memory reg.Store for tests.
package regtest

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"regclean/internal/reg"
)

// Export records one call to Store.Export.
type Export struct {
	Hive reg.Hive
	Path string
	File string
}

// Store is an in-memory registry.
type Store struct {
	nodes map[string]*node

	Exports   []Export
	ExportErr error // when set, Export fails with it
}

type node struct {
	path   string
	values map[string]value // lower-case name -> value
}

type value struct {
	name string
	data any // string or uint64
}

// New returns an empty registry.
func New() *Store { return &Store{nodes: map[string]*node{}} }

func id(h reg.Hive, path string) string { return h.String() + "|" + strings.ToLower(path) }

func (s *Store) ensure(h reg.Hive, path string) *node {
	var n *node
	parts := strings.Split(path, `\`)
	for i := range parts {
		p := strings.Join(parts[:i+1], `\`)
		k := id(h, p)
		if s.nodes[k] == nil {
			s.nodes[k] = &node{path: p, values: map[string]value{}}
		}
		n = s.nodes[k]
	}
	return n
}

// Set creates the key (and parents) if needed and sets a value.
// v may be a string, an int or a uint64 (stored as DWORD).
func (s *Store) Set(h reg.Hive, path, name string, v any) {
	if i, ok := v.(int); ok {
		v = uint64(i)
	}
	s.ensure(h, path).values[strings.ToLower(name)] = value{name: name, data: v}
}

// HasKey reports whether the key exists.
func (s *Store) HasKey(h reg.Hive, path string) bool { return s.nodes[id(h, path)] != nil }

// HasValue reports whether the value exists.
func (s *Store) HasValue(h reg.Hive, path, name string) bool {
	n := s.nodes[id(h, path)]
	if n == nil {
		return false
	}
	_, ok := n.values[strings.ToLower(name)]
	return ok
}

func (s *Store) Open(h reg.Hive, path string) (reg.Key, error) {
	if s.nodes[id(h, path)] == nil {
		return nil, fmt.Errorf("%s\\%s: %w", h, path, reg.ErrNotExist)
	}
	return &key{s: s, h: h, path: path}, nil
}

func (s *Store) DeleteValue(h reg.Hive, path, name string) error {
	n := s.nodes[id(h, path)]
	if n == nil {
		return fmt.Errorf("%s\\%s: %w", h, path, reg.ErrNotExist)
	}
	l := strings.ToLower(name)
	if _, ok := n.values[l]; !ok {
		return fmt.Errorf("value %q: %w", name, reg.ErrNotExist)
	}
	delete(n.values, l)
	return nil
}

func (s *Store) DeleteKey(h reg.Hive, path string) error {
	if s.nodes[id(h, path)] == nil {
		return fmt.Errorf("%s\\%s: %w", h, path, reg.ErrNotExist)
	}
	prefix := strings.ToLower(path) + `\`
	for k, n := range s.nodes {
		if !strings.HasPrefix(k, h.String()+"|") {
			continue
		}
		if k == id(h, path) || strings.HasPrefix(strings.ToLower(n.path), prefix) {
			delete(s.nodes, k)
		}
	}
	return nil
}

func (s *Store) Export(h reg.Hive, path, file string) error {
	if s.ExportErr != nil {
		return s.ExportErr
	}
	s.Exports = append(s.Exports, Export{Hive: h, Path: path, File: file})
	return os.WriteFile(file, []byte("fake export of "+h.String()+`\`+path), 0o644)
}

type key struct {
	s    *Store
	h    reg.Hive
	path string
}

func (k *key) node() *node { return k.s.nodes[id(k.h, k.path)] }

func (k *key) String(name string) (string, bool) {
	if n := k.node(); n != nil {
		if v, ok := n.values[strings.ToLower(name)]; ok {
			str, isStr := v.data.(string)
			return str, isStr
		}
	}
	return "", false
}

func (k *key) DWord(name string) (uint64, bool) {
	if n := k.node(); n != nil {
		if v, ok := n.values[strings.ToLower(name)]; ok {
			d, isNum := v.data.(uint64)
			return d, isNum
		}
	}
	return 0, false
}

func (k *key) ValueNames() ([]string, error) {
	var names []string
	if n := k.node(); n != nil {
		for _, v := range n.values {
			names = append(names, v.name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (k *key) SubKeyNames() ([]string, error) {
	prefix := strings.ToLower(k.path) + `\`
	var names []string
	for key, n := range k.s.nodes {
		if !strings.HasPrefix(key, k.h.String()+"|") || !strings.HasPrefix(strings.ToLower(n.path), prefix) {
			continue
		}
		if rest := n.path[len(prefix):]; !strings.Contains(rest, `\`) {
			names = append(names, rest)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (k *key) Close() error { return nil }
