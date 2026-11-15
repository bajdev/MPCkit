// Package xpj reads and writes MPC Sample project files (.xpj).
//
// A project file is a gzip stream holding five header lines and one JSON
// document. The JSON is held as an ordered tree: objects keep their key order
// and numbers keep their original text, so a file that is read and written
// back unchanged is byte-identical after decompression. The package knows
// nothing about pads, samples or sequences.
package xpj

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Kind is the JSON type of a Node.
type Kind int

const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
)

func (k Kind) String() string {
	switch k {
	case Null:
		return "null"
	case Bool:
		return "bool"
	case Number:
		return "number"
	case String:
		return "string"
	case Array:
		return "array"
	case Object:
		return "object"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Node is one JSON value.
type Node struct {
	Kind Kind
	// Text is the decoded value of a String, or the literal text of a Number.
	Text    string
	Bool    bool
	Items   []*Node  // Array elements
	Members []Member // Object members, in file order
}

// Member is one key/value pair of an object.
type Member struct {
	Key   string
	Value *Node
}

// NewNull returns a null node.
func NewNull() *Node { return &Node{Kind: Null} }

// NewBool returns a boolean node.
func NewBool(b bool) *Node { return &Node{Kind: Bool, Bool: b} }

// NewString returns a string node.
func NewString(s string) *Node { return &Node{Kind: String, Text: s} }

// NewInt returns a number node written as an integer, e.g. 4.
func NewInt(i int64) *Node { return &Node{Kind: Number, Text: strconv.FormatInt(i, 10)} }

// NewFloat returns a number node for a float field. It is always written with
// a decimal point or exponent (96.0, never 96), in the shortest form that
// reads back as the same float64. It panics on NaN or infinity, which JSON
// cannot hold.
func NewFloat(f float64) *Node { return &Node{Kind: Number, Text: FormatFloat(f)} }

// NewFloat32 is NewFloat for a field the device stores as a 32-bit float:
// the value is rounded to float32 first, then printed as a float64
// (12.4 is written as 12.399999618530273).
func NewFloat32(f float64) *Node { return NewFloat(float64(float32(f))) }

// NewArray returns an array node holding items.
func NewArray(items ...*Node) *Node { return &Node{Kind: Array, Items: items} }

// NewObject returns an empty object node.
func NewObject() *Node { return &Node{Kind: Object} }

// FormatFloat prints f the way the reference writer (Python's repr) does:
// shortest round-trip digits, plain notation for exponents from -4 to 15
// with at least one digit after the point, otherwise d.ddde±XX.
func FormatFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		panic("xpj: cannot write NaN or infinity")
	}
	s := strconv.FormatFloat(f, 'e', -1, 64) // e.g. -1.2345e+01
	mant, expStr, _ := strings.Cut(s, "e")
	exp, _ := strconv.Atoi(expStr)
	neg := strings.HasPrefix(mant, "-")
	digits := strings.Replace(strings.TrimPrefix(mant, "-"), ".", "", 1)

	var out string
	if exp < -4 || exp >= 16 {
		out = digits[:1]
		if len(digits) > 1 {
			out += "." + digits[1:]
		}
		sign := "+"
		if exp < 0 {
			sign, exp = "-", -exp
		}
		out += fmt.Sprintf("e%s%02d", sign, exp)
	} else if exp < 0 {
		out = "0." + strings.Repeat("0", -exp-1) + digits
	} else if exp+1 >= len(digits) {
		out = digits + strings.Repeat("0", exp+1-len(digits)) + ".0"
	} else {
		out = digits[:exp+1] + "." + digits[exp+1:]
	}
	if neg {
		out = "-" + out
	}
	return out
}

// errMissing is returned by the value getters for a nil node.
var errMissing = errors.New("xpj: value is missing")

// Int returns the value of a Number node as an integer. A nil node, as
// Lookup returns for a missing key, is an error.
func (n *Node) Int() (int64, error) {
	if n == nil {
		return 0, errMissing
	}
	if n.Kind != Number {
		return 0, fmt.Errorf("xpj: %s is not a number", n.Kind)
	}
	if i, err := strconv.ParseInt(n.Text, 10, 64); err == nil {
		return i, nil
	}
	f, err := strconv.ParseFloat(n.Text, 64)
	if err != nil || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return 0, fmt.Errorf("xpj: %s is not an integer", n.Text)
	}
	return int64(f), nil
}

// Float returns the value of a Number node. A nil node is an error.
func (n *Node) Float() (float64, error) {
	if n == nil {
		return 0, errMissing
	}
	if n.Kind != Number {
		return 0, fmt.Errorf("xpj: %s is not a number", n.Kind)
	}
	return strconv.ParseFloat(n.Text, 64)
}

// Lookup returns the value of the first member called key, or nil if n is not
// an object or has no such member.
func (n *Node) Lookup(key string) *Node {
	if n == nil || n.Kind != Object {
		return nil
	}
	for _, m := range n.Members {
		if m.Key == key {
			return m.Value
		}
	}
	return nil
}

// SetKey replaces the value of the first member called key, keeping its
// position, or appends a new member if there is none.
func (n *Node) SetKey(key string, v *Node) error {
	if n.Kind != Object {
		return fmt.Errorf("xpj: cannot set key %q on %s", key, n.Kind)
	}
	for i := range n.Members {
		if n.Members[i].Key == key {
			n.Members[i].Value = v
			return nil
		}
	}
	n.Members = append(n.Members, Member{Key: key, Value: v})
	return nil
}

// DeleteKey removes the first member called key and reports whether it was there.
func (n *Node) DeleteKey(key string) bool {
	if n.Kind != Object {
		return false
	}
	for i, m := range n.Members {
		if m.Key == key {
			n.Members = append(n.Members[:i], n.Members[i+1:]...)
			return true
		}
	}
	return false
}

// Insert puts v into an array at index i (0 <= i <= len), shifting later items.
func (n *Node) Insert(i int, v *Node) error {
	if n.Kind != Array {
		return fmt.Errorf("xpj: cannot insert into %s", n.Kind)
	}
	if i < 0 || i > len(n.Items) {
		return fmt.Errorf("xpj: insert index %d out of range [0,%d]", i, len(n.Items))
	}
	n.Items = append(n.Items, nil)
	copy(n.Items[i+1:], n.Items[i:])
	n.Items[i] = v
	return nil
}

// Append adds v to the end of an array.
func (n *Node) Append(v *Node) error { return n.Insert(len(n.Items), v) }

// Remove deletes the array item at index i.
func (n *Node) Remove(i int) error {
	if n.Kind != Array {
		return fmt.Errorf("xpj: cannot remove from %s", n.Kind)
	}
	if i < 0 || i >= len(n.Items) {
		return fmt.Errorf("xpj: remove index %d out of range [0,%d)", i, len(n.Items))
	}
	n.Items = append(n.Items[:i], n.Items[i+1:]...)
	return nil
}

// Clone returns a deep copy of n.
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}
	c := &Node{Kind: n.Kind, Text: n.Text, Bool: n.Bool}
	if n.Items != nil {
		c.Items = make([]*Node, len(n.Items))
		for i, it := range n.Items {
			c.Items[i] = it.Clone()
		}
	}
	if n.Members != nil {
		c.Members = make([]Member, len(n.Members))
		for i, m := range n.Members {
			c.Members[i] = Member{Key: m.Key, Value: m.Value.Clone()}
		}
	}
	return c
}
