package xpj

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ParseJSON builds a tree from one JSON document. Numbers keep their text.
func ParseJSON(body []byte) (*Node, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	root, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			err = errors.New("unexpected data after the document")
		}
		return nil, fmt.Errorf("xpj: %w", err)
	}
	return root, nil
}

func parseValue(dec *json.Decoder) (*Node, error) {
	tok, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("xpj: %w", err)
	}
	switch t := tok.(type) {
	case nil:
		return NewNull(), nil
	case bool:
		return NewBool(t), nil
	case json.Number:
		return &Node{Kind: Number, Text: string(t)}, nil
	case string:
		return NewString(t), nil
	case json.Delim:
		switch t {
		case '[':
			n := &Node{Kind: Array, Items: []*Node{}}
			for dec.More() {
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				n.Items = append(n.Items, v)
			}
			_, err := dec.Token() // ]
			return n, wrapErr(err)
		case '{':
			n := &Node{Kind: Object, Members: []Member{}}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, wrapErr(err)
				}
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				n.Members = append(n.Members, Member{Key: kt.(string), Value: v})
			}
			_, err := dec.Token() // }
			return n, wrapErr(err)
		}
	}
	return nil, fmt.Errorf("xpj: unexpected token %v", tok)
}

// wrapErr prefixes a JSON decoding error with the package name.
func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("xpj: %w", err)
}

// MarshalJSON writes n in the device's layout: one key or value per line, no
// indentation, ": " after keys, "," at line ends, empty containers as [] and
// {}, no trailing newline.
func (n *Node) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	if err := writeValue(&b, n); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func writeValue(b *bytes.Buffer, n *Node) error {
	if n == nil {
		return errors.New("xpj: nil node in tree")
	}
	switch n.Kind {
	case Null:
		b.WriteString("null")
	case Bool:
		if n.Bool {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case Number:
		if !isNumber(n.Text) {
			return fmt.Errorf("xpj: invalid number text %q", n.Text)
		}
		b.WriteString(n.Text)
	case String:
		writeString(b, n.Text)
	case Array:
		if len(n.Items) == 0 {
			b.WriteString("[]")
			return nil
		}
		b.WriteString("[\n")
		for i, it := range n.Items {
			if i > 0 {
				b.WriteString(",\n")
			}
			if err := writeValue(b, it); err != nil {
				return err
			}
		}
		b.WriteString("\n]")
	case Object:
		if len(n.Members) == 0 {
			b.WriteString("{}")
			return nil
		}
		b.WriteString("{\n")
		for i, m := range n.Members {
			if i > 0 {
				b.WriteString(",\n")
			}
			writeString(b, m.Key)
			b.WriteString(": ")
			if err := writeValue(b, m.Value); err != nil {
				return err
			}
		}
		b.WriteString("\n}")
	default:
		return fmt.Errorf("xpj: unknown node kind %d", n.Kind)
	}
	return nil
}

// writeString escapes like the reference writer (Python json, ensure_ascii
// off): quote, backslash and control characters only. '<', '>', '&', '/' and
// non-ASCII text are written as they are.
func writeString(b *bytes.Buffer, s string) {
	const hex = "0123456789abcdef"
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if c < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&0xf])
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
}

// isNumber reports whether s is a JSON number literal.
func isNumber(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	switch {
	case i < len(s) && s[i] == '0':
		i++
	case i < len(s) && s[i] >= '1' && s[i] <= '9':
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	default:
		return false
	}
	if i < len(s) && s[i] == '.' {
		i++
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
	}
	return i == len(s)
}
