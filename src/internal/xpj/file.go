package xpj

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// HeaderLines is the number of text lines before the JSON body.
const HeaderLines = 5

// Header values seen in every file saved by firmware 1.3.0.12.
var TestedHeader = []string{"ACVS", "1.3.0.12", "SerialisableProjectData", "json", "Linux"}

// File is a decoded project file.
type File struct {
	Header []string // the five header lines, without newlines
	Root   *Node    // the JSON document, {"data": {...}}
}

// Decode parses a decompressed project file.
func Decode(raw []byte) (*File, error) {
	parts := bytes.SplitN(raw, []byte("\n"), HeaderLines+1)
	if len(parts) != HeaderLines+1 {
		return nil, errors.New("xpj: file is shorter than its five header lines")
	}
	header := make([]string, HeaderLines)
	for i := range header {
		header[i] = string(parts[i])
	}
	if header[0] != "ACVS" {
		return nil, fmt.Errorf("xpj: not an MPC project (first line %q, want ACVS)", header[0])
	}
	root, err := ParseJSON(parts[HeaderLines])
	if err != nil {
		return nil, err
	}
	if root.Kind != Object {
		return nil, fmt.Errorf("xpj: body is %s, want object", root.Kind)
	}
	return &File{Header: header, Root: root}, nil
}

// Encode returns the decompressed file: header lines, then the JSON body.
func (f *File) Encode() ([]byte, error) {
	if len(f.Header) != HeaderLines {
		return nil, fmt.Errorf("xpj: header has %d lines, want %d", len(f.Header), HeaderLines)
	}
	for _, h := range f.Header {
		if strings.ContainsAny(h, "\r\n") {
			return nil, fmt.Errorf("xpj: header line %q contains a newline", h)
		}
	}
	body, err := f.Root.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Grow(len(body) + 64)
	for _, h := range f.Header {
		b.WriteString(h)
		b.WriteByte('\n')
	}
	b.Write(body)
	return b.Bytes(), nil
}

// MaxDecompressed is the largest project Read accepts, in decompressed
// bytes. A small file can decompress to gigabytes (a 1 MB file padded with
// spaces used 3.1 GB), so Read stops there instead of running out of
// memory. Device projects are about 3 MB; 256 MB holds about half a
// million recorded events.
const MaxDecompressed = 256 << 20

// Read decompresses and decodes a project file.
func Read(r io.Reader) (*File, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("xpj: not a gzip stream: %w", err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(io.LimitReader(zr, MaxDecompressed+1))
	if err != nil {
		return nil, fmt.Errorf("xpj: decompress: %w", err)
	}
	if len(raw) > MaxDecompressed {
		return nil, fmt.Errorf("xpj: project is larger than %d MB when decompressed", MaxDecompressed>>20)
	}
	return Decode(raw)
}

// Write encodes f and writes it gzip-compressed, with the gzip header the
// device and the reference writer use: no name, modification time 0, OS Unix.
func (f *File) Write(w io.Writer) error {
	raw, err := f.Encode()
	if err != nil {
		return err
	}
	zw, err := gzip.NewWriterLevel(w, 6)
	if err != nil {
		return err
	}
	zw.OS = 3
	if _, err := zw.Write(raw); err != nil {
		return err
	}
	return zw.Close()
}

// ReadFile reads a project file from disk.
func ReadFile(path string) (*File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return Read(fh)
}

// WriteFile writes a project file to disk.
func (f *File) WriteFile(path string) error {
	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// Data returns the "data" object that holds the whole project.
func (f *File) Data() (*Node, error) {
	d := f.Root.Lookup("data")
	if d == nil || d.Kind != Object {
		return nil, errors.New("xpj: no data object")
	}
	return d, nil
}

// Clone returns a deep copy of f.
func (f *File) Clone() *File {
	return &File{Header: append([]string(nil), f.Header...), Root: f.Root.Clone()}
}
