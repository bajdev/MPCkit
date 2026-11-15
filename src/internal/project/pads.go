package project

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/bajdev/MPCkit/internal/wav"
	"github.com/bajdev/MPCkit/internal/xpj"
)

// Values the device writes when it assigns a sample with no tempo or key of
// its own (format doc section 3, build plan 4.3).
const (
	padTempo        = 20.0
	newSampleKey    = "C Natural Minor"
	newSampleRoot   = 60
	crossfadeLength = -1
)

// Pad is a read-only view of one pad.
type Pad struct {
	Slot       int
	Label      string // "A1" .. "H16"
	SampleName string // empty for an empty pad
	File       string // pool file name
	Coarse     int
	Fine       int
	MuteGroup  int
	Start, End int64 // slice start and end frame, for the preview
}

// Empty reports whether the pad has no sample.
func (p Pad) Empty() bool { return p.File == "" }

// Pad returns the pad in slot.
func (p *Project) Pad(slot int) (Pad, error) {
	if err := checkSlot(slot); err != nil {
		return Pad{}, err
	}
	pad := p.pad(slot)
	l, err := layer(pad)
	if err != nil {
		return Pad{}, fmt.Errorf("project: pad %s: %w", SlotName(slot), err)
	}
	v := Pad{Slot: slot, Label: SlotName(slot)}
	v.SampleName = text(l.Lookup("sampleName"))
	v.File = text(l.Lookup("sampleFile"))
	v.Coarse = int(num(l.Lookup("coarseTune")))
	v.Fine = int(num(l.Lookup("fineTune")))
	v.MuteGroup = int(num(pad.Lookup("whichMuteGroup")))
	if si := l.Lookup("sliceInfo"); si != nil {
		v.Start = num(si.Lookup("Start"))
		v.End = num(si.Lookup("End"))
	}
	return v, nil
}

func text(n *xpj.Node) string {
	if n == nil || n.Kind != xpj.String {
		return ""
	}
	return n.Text
}

// num and float return a number field's value, or 0 if it is missing or
// not a number of that kind.
func num(n *xpj.Node) int64 {
	i, _ := n.Int()
	return i
}

func float(n *xpj.Node) float64 {
	f, _ := n.Float()
	return f
}

// padFiles returns the pool files used by any layer of pad.
func padFiles(pad *xpj.Node) []string {
	var files []string
	layers := pad.Lookup("layersv")
	if layers == nil {
		return nil
	}
	for _, l := range layers.Items {
		if f := text(l.Lookup("sampleFile")); f != "" {
			files = append(files, f)
		}
	}
	return files
}

// usage counts the pad layers that use the pool file.
func (p *Project) usage(file string) int {
	n := 0
	for _, pad := range p.pads().Items {
		for _, f := range padFiles(pad) {
			if f == file {
				n++
			}
		}
	}
	return n
}

// SampleCount returns the number of pool entries.
func (p *Project) SampleCount() int { return len(p.pool().Items) }

// AssignSample puts the WAV at src on the pad in slot, replacing what was
// there. A file already in the pool from the same source is reused; a
// different file with a name already in the pool gets a 001, 002 suffix.
func (p *Project) AssignSample(slot int, src string) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	abs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	name := filepath.Base(abs)
	// Export refuses a source that is a link, so that a link planted in a
	// shared kit's data folder cannot pull in other files. A link the user
	// picks themselves is fine: record the file it points to.
	target, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	info, err := wav.ReadFile(target)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if err := info.Validate(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	// Work out every change before making any, so that a failure leaves
	// the project as it was. Reuse the pool entry already copied from this
	// file, even if this pad was its only user.
	file := p.fileForSource(target)
	freed := p.freedBy(slot, file)
	newFile := file
	if file == "" {
		if p.SampleCount()-len(freed) >= MaxSamples {
			return fmt.Errorf("the project already has %d samples, the most the device allows", MaxSamples)
		}
		newFile = p.uniqueFile(poolFileName(name), freed)
	}
	pad := template.pad.Clone()
	l, err := layer(pad)
	if err != nil {
		return err
	}
	var e setter
	e.set(pad, "tempo", xpj.NewFloat(padTempo))
	e.set(l, "sampleName", xpj.NewString(stem(newFile)))
	e.set(l, "sampleFile", xpj.NewString(newFile))
	e.set(l, "sliceInfo.End", xpj.NewInt(info.Frames-1))
	e.set(l, "sliceInfo.LoopCrossfadeLength", xpj.NewInt(crossfadeLength))
	if e.err != nil {
		return e.err
	}

	// Apply. None of these steps can fail: fromFile checked that the pool
	// and the pads are lists.
	p.clear(slot, file)
	p.pads().Items[slot] = pad
	if file == "" {
		p.addPoolEntry(newFile)
		p.sources[newFile] = target
	}
	p.dirty = true
	return nil
}

// freedBy returns the pool files that clearing the pad in slot would drop:
// those no other pad uses, except keep.
func (p *Project) freedBy(slot int, keep string) map[string]bool {
	here := map[string]int{} // layers of this pad using each file
	for _, f := range padFiles(p.pad(slot)) {
		here[f]++
	}
	freed := map[string]bool{}
	for f, n := range here {
		if f != keep && p.usage(f) == n {
			freed[f] = true
		}
	}
	return freed
}

// fileForSource returns the pool file already copied from src, if any.
func (p *Project) fileForSource(src string) string {
	for file, s := range p.sources {
		if s == src && p.inPool(file) {
			return file
		}
	}
	return ""
}

func (p *Project) inPool(file string) bool {
	for _, e := range p.pool().Items {
		if text(e.Lookup("path")) == file {
			return true
		}
	}
	return false
}

// poolFileName turns the name of a WAV the user picked into a pool file
// name that passes PlainFileName on every system and is legal on the
// device's FAT card. Linux and macOS allow names such as "kick\one.wav",
// "a:b.wav" or "con.wav"; exported as they are, the backslash reads as a
// folder on Windows and the card, the colon cannot be written to the card,
// Windows cannot create a file named after a device, and Load refuses the
// project. Each such character becomes "_", and a device name gets "_"
// added ("con.wav" becomes "con_.wav").
func poolFileName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	if clean == "" || clean == "." || clean == ".." {
		return "sample.wav"
	}
	if reservedOnWindows(clean) {
		base, ext, _ := strings.Cut(clean, ".")
		clean = base + "_"
		if ext != "" {
			clean += "." + ext
		}
	}
	return clean
}

func stem(file string) string { return strings.TrimSuffix(file, filepath.Ext(file)) }

// uniqueFile returns base, or base with a 001, 002 ... suffix before the
// extension, so that no two pool files share a file name or a sample name
// (the file name without its extension). Names are compared without case,
// as on Windows, macOS and FAT.
func (p *Project) uniqueFile(base string, freed map[string]bool) string {
	takenFiles := map[string]bool{}
	takenNames := map[string]bool{}
	for _, e := range p.pool().Items {
		if freed[text(e.Lookup("path"))] {
			continue // about to be dropped
		}
		takenFiles[strings.ToLower(text(e.Lookup("path")))] = true
		takenNames[strings.ToLower(text(e.Lookup("name")))] = true
	}
	free := func(f string) bool {
		return !takenFiles[strings.ToLower(f)] && !takenNames[strings.ToLower(stem(f))]
	}
	if free(base) {
		return base
	}
	ext := filepath.Ext(base)
	for i := 1; ; i++ {
		f := fmt.Sprintf("%s%03d%s", stem(base), i, ext)
		if free(f) {
			return f
		}
	}
}

// poolEntry builds a new pool entry in the device's key order.
func poolEntry(file string) *xpj.Node {
	meta := xpj.NewObject()
	meta.Members = []xpj.Member{
		{Key: "tempo", Value: xpj.NewFloat(0)},
		{Key: "rootNote", Value: xpj.NewInt(newSampleRoot)},
		{Key: "tune", Value: xpj.NewFloat(0)},
		{Key: "key", Value: xpj.NewString(newSampleKey)},
	}
	e := xpj.NewObject()
	e.Members = []xpj.Member{
		{Key: "version", Value: xpj.NewInt(1)},
		{Key: "name", Value: xpj.NewString(stem(file))},
		{Key: "path", Value: xpj.NewString(file)},
		{Key: "loadImpl", Value: xpj.NewInt(0)},
		{Key: "metadata", Value: meta},
	}
	return e
}

// addPoolEntry adds file to both pools. Both are lists (checked by fromFile),
// so this cannot fail.
func (p *Project) addPoolEntry(file string) {
	_ = p.pool().Append(poolEntry(file))
	_ = p.trackPool().Insert(0, poolEntry(file))
}

func (p *Project) removePoolEntry(file string) {
	for _, arr := range []*xpj.Node{p.pool(), p.trackPool()} {
		for i := len(arr.Items) - 1; i >= 0; i-- {
			if text(arr.Items[i].Lookup("path")) == file {
				_ = arr.Remove(i)
			}
		}
	}
	delete(p.sources, file)
}

// ClearPad resets the pad in slot to the blank pad and drops pool entries
// no other pad uses.
func (p *Project) ClearPad(slot int) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	if len(padFiles(p.pad(slot))) == 0 {
		return nil
	}
	p.clear(slot, "")
	p.dirty = true
	return nil
}

// clear resets the pad in slot and drops pool entries left unused, except keep.
func (p *Project) clear(slot int, keep string) {
	files := padFiles(p.pad(slot))
	p.pads().Items[slot] = template.pad.Clone()
	for _, f := range files {
		if f != keep && p.usage(f) == 0 {
			p.removePoolEntry(f)
		}
	}
}

// CopyPad copies the whole pad in from onto to, replacing what was there.
// Both pads share the pool entry.
func (p *Project) CopyPad(from, to int) error {
	if err := checkSlot(from); err != nil {
		return err
	}
	if err := checkSlot(to); err != nil {
		return err
	}
	if from == to {
		return nil
	}
	src := p.pad(from).Clone()
	p.clear(to, "")
	p.pads().Items[to] = src
	p.dirty = true
	return nil
}

// MovePad copies from onto to, then resets from to the blank pad. Recorded
// notes stay on their slot; check PadHasNotes and warn first.
func (p *Project) MovePad(from, to int) error {
	if from == to {
		return checkSlot(from)
	}
	if err := p.CopyPad(from, to); err != nil {
		return err
	}
	p.pads().Items[from] = template.pad.Clone()
	return nil
}

// SwapPads exchanges two pads.
func (p *Project) SwapPads(a, b int) error {
	if err := checkSlot(a); err != nil {
		return err
	}
	if err := checkSlot(b); err != nil {
		return err
	}
	if a == b {
		return nil
	}
	items := p.pads().Items
	items[a], items[b] = items[b], items[a]
	p.dirty = true
	return nil
}

// SetTune sets the pad's tuning, clamped to -24..+24 semitones and
// -90..+90 cents. pitch is stored as a 32-bit float.
func (p *Project) SetTune(slot, coarse, fine int) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	coarse = max(MinCoarse, min(MaxCoarse, coarse))
	fine = max(MinFine, min(MaxFine, fine))
	l, err := layer(p.pad(slot))
	if err != nil {
		return err
	}
	if l.Kind != xpj.Object {
		return fmt.Errorf("project: pad %s layer 1 is %s, not an object", SlotName(slot), l.Kind)
	}
	// Setting keys on an object cannot fail, so all three are written.
	var e setter
	e.set(l, "coarseTune", xpj.NewInt(int64(coarse)))
	e.set(l, "fineTune", xpj.NewInt(int64(fine)))
	e.set(l, "pitch", xpj.NewFloat32(float64(coarse)+float64(fine)/100))
	if e.err != nil {
		return e.err
	}
	p.dirty = true
	return nil
}

// SetMuteGroup sets the pad's mute group; 0 is off.
func (p *Project) SetMuteGroup(slot, group int) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	if group < 0 || group > MaxMuteGroup {
		return fmt.Errorf("mute group %d out of range 0-%d", group, MaxMuteGroup)
	}
	if err := p.pad(slot).SetKey("whichMuteGroup", xpj.NewInt(int64(group))); err != nil {
		return fmt.Errorf("project: pad %s: %w", SlotName(slot), err)
	}
	p.dirty = true
	return nil
}

// PadHasNotes reports whether any sequence has a note or automation event
// for the pad in slot (note 36 + slot on the drum track).
func (p *Project) PadHasNotes(slot int) bool {
	note := int64(FirstNote + slot)
	found := false
	p.eachDrumEvent(func(ev *xpj.Node) bool {
		var n *xpj.Node
		switch num(ev.Lookup("type")) {
		case eventNote:
			n = ev.Lookup("note")
		case eventAutomation:
			n = ev.Lookup("automation")
		}
		if n != nil && num(n.Lookup("note")) == note {
			found = true
		}
		return !found
	})
	return found
}

// PoolFiles returns the file name of every pool entry, in pool order.
func (p *Project) PoolFiles() []string {
	var files []string
	for _, e := range p.pool().Items {
		files = append(files, text(e.Lookup("path")))
	}
	return files
}
