package xpj

import (
	"fmt"
	"strconv"
	"strings"
)

// A path names a node below another, in the form used by the format doc:
// object keys separated by dots, array indexes in brackets, e.g.
// "tracks[0].program.drum.instruments[4].layersv[0].sliceInfo.End".
// Keys containing '.', '[' or ']' cannot be addressed this way; use Lookup.

type step struct {
	key   string
	index int // -1 for a key step
}

func parsePath(path string) ([]step, error) {
	var steps []step
	rest := path
	for rest != "" {
		if rest[0] == '[' {
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				return nil, fmt.Errorf("xpj: path %q: missing ]", path)
			}
			i, err := strconv.Atoi(rest[1:end])
			if err != nil || i < 0 {
				return nil, fmt.Errorf("xpj: path %q: bad index %q", path, rest[1:end])
			}
			steps = append(steps, step{index: i})
			rest = rest[end+1:]
			if strings.HasPrefix(rest, ".") {
				rest = rest[1:]
				if rest == "" || rest[0] == '[' {
					return nil, fmt.Errorf("xpj: path %q: empty key", path)
				}
			}
			continue
		}
		end := strings.IndexAny(rest, ".[]")
		if end < 0 {
			end = len(rest)
		}
		if end == 0 {
			return nil, fmt.Errorf("xpj: path %q: empty key", path)
		}
		steps = append(steps, step{key: rest[:end], index: -1})
		rest = rest[end:]
		if strings.HasPrefix(rest, "]") {
			return nil, fmt.Errorf("xpj: path %q: unexpected ]", path)
		}
		if strings.HasPrefix(rest, ".") {
			rest = rest[1:]
			if rest == "" || rest[0] == '[' {
				return nil, fmt.Errorf("xpj: path %q: empty key", path)
			}
		}
	}
	return steps, nil
}

func (n *Node) walk(steps []step, path string) (*Node, error) {
	cur := n
	if cur == nil {
		return nil, fmt.Errorf("xpj: path %q: node is missing", path)
	}
	for i, s := range steps {
		if s.index < 0 {
			next := cur.Lookup(s.key)
			if next == nil {
				return nil, fmt.Errorf("xpj: path %q: no key %q at %s", path, s.key, describe(steps[:i]))
			}
			cur = next
			continue
		}
		if cur.Kind != Array {
			return nil, fmt.Errorf("xpj: path %q: %s is %s, not an array", path, describe(steps[:i]), cur.Kind)
		}
		if s.index >= len(cur.Items) {
			return nil, fmt.Errorf("xpj: path %q: index %d out of range at %s (length %d)", path, s.index, describe(steps[:i]), len(cur.Items))
		}
		cur = cur.Items[s.index]
	}
	return cur, nil
}

func describe(steps []step) string {
	if len(steps) == 0 {
		return "root"
	}
	var b strings.Builder
	for i, s := range steps {
		if s.index >= 0 {
			fmt.Fprintf(&b, "[%d]", s.index)
			continue
		}
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s.key)
	}
	return b.String()
}

// Get returns the node at path below n. An empty path returns n.
func (n *Node) Get(path string) (*Node, error) {
	steps, err := parsePath(path)
	if err != nil {
		return nil, err
	}
	return n.walk(steps, path)
}

// Set replaces the node at path below n. The parent must exist; a missing
// final key is appended to its object, but array indexes must already exist.
func (n *Node) Set(path string, v *Node) error {
	steps, err := parsePath(path)
	if err != nil {
		return err
	}
	if len(steps) == 0 {
		return fmt.Errorf("xpj: cannot Set the root")
	}
	parent, err := n.walk(steps[:len(steps)-1], path)
	if err != nil {
		return err
	}
	last := steps[len(steps)-1]
	if last.index < 0 {
		return parent.SetKey(last.key, v)
	}
	if parent.Kind != Array {
		return fmt.Errorf("xpj: path %q: parent is %s, not an array", path, parent.Kind)
	}
	if last.index >= len(parent.Items) {
		return fmt.Errorf("xpj: path %q: index %d out of range (length %d)", path, last.index, len(parent.Items))
	}
	parent.Items[last.index] = v
	return nil
}
