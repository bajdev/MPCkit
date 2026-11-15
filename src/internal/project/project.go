// Package project is the MPCkit domain model: one MPC Sample project and
// every edit 0.5.0 makes to it. Edits follow the verified recipes in
// docs/xpj-format-and-scope.md section 3 and change only the fields those
// recipes name; everything else is written back as read.
package project

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"git.monoexp.com/bajdev/mpckit/internal/xpj"
)

//go:embed template/DemoBlank.xpj
var templateXPJ []byte

// Device and format limits.
const (
	Pads          = 128 // 8 banks x 16
	MaxSamples    = 128
	MaxSequences  = 128
	PulsesPerBar  = 3840 // 4/4 at 960 pulses per beat
	FirstNote     = 36   // a note event for pad slot n stores note 36 + n
	TestedVersion = 28   // data.version of firmware 1.3.0.12
	DataSuffix    = "_[ProjectData]"
)

// Tuning ranges for 0.5.0 (format doc, open question 1a).
const (
	MinCoarse, MaxCoarse = -24, 24
	MinFine, MaxFine     = -90, 90
)

// MaxMuteGroup is the highest mute group the device offers; 0 is off.
// Device checkpoint A3, 2026-11-10.
const MaxMuteGroup = 16

// BPM range of the device's tempo control (device checkpoint A5, 2026-11-10).
const (
	MinBPM = 20.0
	MaxBPM = 999.0
)

// DefaultBPM is used for a new sequence when no sequence has been edited.
const DefaultBPM = 120.0

// Project is one project: the parsed file plus what the file does not hold.
type Project struct {
	// Name is the project name: the .xpj file name and data folder prefix.
	Name string
	// SourceDir is the data folder of a loaded project; empty for a new one.
	SourceDir string

	// sources maps each pool file name to the absolute path it is copied
	// from at export.
	sources map[string]string
	// dirty is set by every edit that succeeds; see Dirty.
	dirty bool

	file *xpj.File
	data *xpj.Node
	// The lists every operation works on, found once by fromFile.
	nodes struct{ pool, trackPool, pads, sequences *xpj.Node }

	lastBPM float64 // BPM of the most recently edited sequence; 0 if none
}

var template = mustTemplate()

type tmpl struct {
	pad      *xpj.Node // one blank pad (all 128 are identical)
	sequence *xpj.Node // the blank project's single sequence entry {key, value}
}

func mustTemplate() tmpl {
	f, err := xpj.Read(bytes.NewReader(templateXPJ))
	if err != nil {
		panic("project: embedded template: " + err.Error())
	}
	d, _ := f.Data()
	pad, err := d.Get("tracks[0].program.drum.instruments[0]")
	if err != nil {
		panic("project: embedded template: " + err.Error())
	}
	seq, err := d.Get("sequences[0]")
	if err != nil {
		panic("project: embedded template: " + err.Error())
	}
	return tmpl{pad: pad, sequence: seq}
}

// New returns a new project built from the blank template.
func New(name string) (*Project, error) {
	if err := ValidName(name); err != nil {
		return nil, err
	}
	f, err := xpj.Read(bytes.NewReader(templateXPJ))
	if err != nil {
		return nil, err
	}
	return fromFile(f, name, "")
}

// Load reads the .xpj at path. The project name is taken from the file name
// and samples are expected in the sibling "<name>_[ProjectData]" folder.
// The returned warnings describe differences from the tested firmware;
// the project is still usable.
func Load(path string) (*Project, []string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	f, err := xpj.ReadFile(abs)
	if err != nil {
		return nil, nil, err
	}
	name := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	dataDir := filepath.Join(filepath.Dir(abs), name+DataSuffix)
	p, err := fromFile(f, name, dataDir)
	if err != nil {
		return nil, nil, err
	}

	var warnings []string
	if f.Header[1] != xpj.TestedHeader[1] {
		warnings = append(warnings, fmt.Sprintf("saved by firmware %s; MPCkit is tested with %s", f.Header[1], xpj.TestedHeader[1]))
	}
	if v, err := p.data.Lookup("version").Int(); err != nil {
		warnings = append(warnings, fmt.Sprintf("project data version is missing; MPCkit is tested with %d", TestedVersion))
	} else if v != TestedVersion {
		warnings = append(warnings, fmt.Sprintf("project data version %d; MPCkit is tested with %d", v, TestedVersion))
	}
	for _, file := range p.PoolFiles() {
		p.sources[file] = filepath.Join(dataDir, file)
	}
	return p, warnings, nil
}

// fromFile builds a Project around a decoded file: it finds the lists every
// operation uses and refuses a file the operations could not use safely.
func fromFile(f *xpj.File, name, sourceDir string) (*Project, error) {
	d, err := f.Data()
	if err != nil {
		return nil, err
	}
	p := &Project{Name: name, SourceDir: sourceDir, sources: map[string]string{}, file: f, data: d}
	for _, l := range []struct {
		path string
		node **xpj.Node
	}{
		{"samples", &p.nodes.pool},
		{"tracks[0].samples", &p.nodes.trackPool},
		{"tracks[0].program.drum.instruments", &p.nodes.pads},
		{"sequences", &p.nodes.sequences},
	} {
		n, err := d.Get(l.path)
		if err != nil {
			return nil, fmt.Errorf("project: %w", err)
		}
		if n.Kind != xpj.Array {
			return nil, fmt.Errorf("project: %s is not a list", l.path)
		}
		*l.node = n
	}
	if n := len(p.pads().Items); n != Pads {
		return nil, fmt.Errorf("project: %d pads, want %d", n, Pads)
	}
	if err := p.check(); err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	return p, nil
}

// check refuses a file whose sample names or sequences the operations
// cannot use safely. Pool file names are joined to folders at load and
// export, so a name such as "../x.wav" would reach outside the project.
func (p *Project) check() error {
	pools := []struct {
		path string
		node *xpj.Node
	}{{"samples", p.pool()}, {"tracks[0].samples", p.trackPool()}}
	for _, pool := range pools {
		for i, e := range pool.node.Items {
			at := fmt.Sprintf("%s[%d].path", pool.path, i)
			n := e.Lookup("path")
			if n == nil || n.Kind != xpj.String {
				return fmt.Errorf("%s is missing or not text", at)
			}
			if err := PlainFileName(n.Text); err != nil {
				return fmt.Errorf("%s: %w", at, err)
			}
		}
	}
	for slot, pad := range p.pads().Items {
		layers := pad.Lookup("layersv")
		if layers == nil {
			continue
		}
		for i, l := range layers.Items {
			// An empty name is an empty layer.
			n := l.Lookup("sampleFile")
			if n == nil {
				continue
			}
			at := fmt.Sprintf("pad %s layer %d sampleFile", SlotName(slot), i+1)
			if n.Kind != xpj.String {
				return fmt.Errorf("%s is not text", at)
			}
			if n.Text == "" {
				continue
			}
			if err := PlainFileName(n.Text); err != nil {
				return fmt.Errorf("%s: %w", at, err)
			}
		}
	}
	for i, s := range p.sequences().Items {
		if err := checkSequence(s); err != nil {
			return fmt.Errorf("sequences[%d]: %w", i, err)
		}
	}
	return nil
}

// checkSequence checks the fields Sequences and the sequence edits read.
func checkSequence(s *xpj.Node) error {
	if _, err := s.Lookup("key").Int(); err != nil {
		return errors.New("key is missing or not a whole number")
	}
	v := s.Lookup("value")
	if v == nil || v.Kind != xpj.Object {
		return errors.New("value is missing or not an object")
	}
	if _, err := v.Lookup("bpm").Float(); err != nil {
		return errors.New("bpm is missing or not a number")
	}
	if _, err := v.Lookup("lengthBars").Int(); err != nil {
		return errors.New("lengthBars is missing or not a whole number")
	}
	if n := v.Lookup("name"); n == nil || n.Kind != xpj.String {
		return errors.New("name is missing or not text")
	}
	return nil
}

// PlainFileName checks that name is a single file name with no folder part,
// safe to join to a folder on any system: not empty, not "." or "..", and
// free of slashes, backslashes and control characters.
func PlainFileName(name string) error {
	switch {
	case name == "":
		return errors.New("file name is empty")
	case name == "." || name == "..":
		return fmt.Errorf("%q is not a file name", name)
	case strings.ContainsAny(name, `/\`):
		return fmt.Errorf("file name %q contains a folder separator", name)
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return fmt.Errorf("file name %q contains a control character", name)
	case filepath.Base(name) != name:
		return fmt.Errorf("%q is not a plain file name", name)
	}
	return nil
}

// Dirty reports whether the project has changes not yet exported.
func (p *Project) Dirty() bool { return p.dirty }

// MarkExported records that the project as it is now has been exported.
func (p *Project) MarkExported() { p.dirty = false }

// Source returns the absolute path the pool file is copied from at export,
// and whether one is recorded.
func (p *Project) Source(file string) (string, bool) {
	src, ok := p.sources[file]
	return src, ok
}

// File returns the project file for writing. Callers must not edit it.
func (p *Project) File() *xpj.File { return p.file }

// SetName renames the project. Nothing inside the file changes.
func (p *Project) SetName(name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	if name != p.Name {
		p.Name = name
		p.dirty = true
	}
	return nil
}

// ValidName checks a project or sequence name: not empty, and free of
// characters that are illegal in file names on Windows or FAT.
func ValidName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is empty")
	}
	for _, r := range name {
		if r < 0x20 || strings.ContainsRune(`<>:"/\|?*`, r) {
			if r < 0x20 {
				return errors.New("name contains a control character")
			}
			return fmt.Errorf("name contains %q, which is not allowed in file names", r)
		}
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return errors.New("name cannot end with a dot or a space")
	}
	if reservedOnWindows(name) {
		return fmt.Errorf("%q is a reserved name on Windows", name)
	}
	return nil
}

// reservedOnWindows reports whether Windows treats name as a device, such as
// "CON" or "com1.wav": the part before the first dot is what counts.
func reservedOnWindows(name string) bool {
	base, _, _ := strings.Cut(strings.ToUpper(name), ".")
	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	}
	return false
}

// Slot helpers. A slot is 0-127: bank x 16 + pad - 1.

// SlotName returns the pad label for a slot, e.g. 0 -> "A1", 17 -> "B2".
func SlotName(slot int) string {
	return fmt.Sprintf("%c%d", 'A'+slot/16, slot%16+1)
}

func checkSlot(slot int) error {
	if slot < 0 || slot >= Pads {
		return fmt.Errorf("pad slot %d out of range 0-%d", slot, Pads-1)
	}
	return nil
}

func (p *Project) pads() *xpj.Node { return p.nodes.pads }

func (p *Project) pad(slot int) *xpj.Node { return p.pads().Items[slot] }

func layer(pad *xpj.Node) (*xpj.Node, error) {
	return pad.Get("layersv[0]")
}

// pool is samples[]; trackPool is tracks[0].samples[]. The device keeps
// the same entries in both, appending to the first and prepending to the
// second.
func (p *Project) pool() *xpj.Node { return p.nodes.pool }

func (p *Project) trackPool() *xpj.Node { return p.nodes.trackPool }

func (p *Project) sequences() *xpj.Node { return p.nodes.sequences }

// setter applies several Sets and keeps the first error, so an operation
// can list its writes plainly. Paths come from the template; a failure means
// the loaded file has a different shape.
type setter struct{ err error }

func (e *setter) set(n *xpj.Node, path string, v *xpj.Node) {
	if e.err == nil {
		if err := n.Set(path, v); err != nil {
			e.err = fmt.Errorf("project: %w", err)
		}
	}
}
