package project

import (
	"errors"
	"fmt"
	"sort"

	"github.com/bajdev/MPCkit/internal/xpj"
)

// Event types in a clip's event list.
const (
	eventAutomation = 1
	eventNote       = 3
)

// MaxBars is the longest sequence MPCkit writes. 64 bars is confirmed on the
// device (checkpoint A5); its own maximum is not yet known.
const MaxBars = 999

// Sequence is a read-only view of one sequence.
type Sequence struct {
	Slot     int
	Name     string
	BPM      float64
	Bars     int
	HasNotes bool
}

// Sequences returns the project's sequences in slot order.
func (p *Project) Sequences() []Sequence {
	var out []Sequence
	for _, s := range p.sequences().Items {
		v := s.Lookup("value")
		out = append(out, Sequence{
			Slot:     int(num(s.Lookup("key"))),
			Name:     text(v.Lookup("name")),
			BPM:      float(v.Lookup("bpm")),
			Bars:     int(num(v.Lookup("lengthBars"))),
			HasNotes: hasNotes(v),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slot < out[j].Slot })
	return out
}

// findSequence returns the index in sequences[] of the sequence in slot, or -1.
func (p *Project) findSequence(slot int) int {
	for i, s := range p.sequences().Items {
		if num(s.Lookup("key")) == int64(slot) {
			return i
		}
	}
	return -1
}

func checkSeqSlot(slot int) error {
	if slot < 0 || slot >= MaxSequences {
		return fmt.Errorf("sequence slot %d out of range 0-%d", slot, MaxSequences-1)
	}
	return nil
}

// AddSequence adds an empty sequence in slot, cloned from the blank project:
// named "Sequence NN", 2 bars, with the BPM of the most recently edited
// sequence (else 120). Like the device, the newest sequence goes first.
func (p *Project) AddSequence(slot int) error {
	if err := checkSeqSlot(slot); err != nil {
		return err
	}
	if p.findSequence(slot) >= 0 {
		return fmt.Errorf("%s already holds a sequence", SlotName(slot))
	}
	bpm := p.lastBPM
	if bpm == 0 {
		bpm = DefaultBPM
	}
	s := template.sequence.Clone()
	var e setter
	e.set(s, "key", xpj.NewInt(int64(slot)))
	v := s.Lookup("value")
	e.set(v, "name", xpj.NewString(fmt.Sprintf("Sequence %02d", slot+1)))
	e.set(v, "bpm", xpj.NewFloat(bpm))
	if e.err != nil {
		return e.err
	}
	if err := setLength(v, 2); err != nil {
		return err
	}
	// The edits above were on a copy; inserting into a list cannot fail.
	_ = p.sequences().Insert(0, s)
	p.dirty = true
	return nil
}

// RemoveSequence deletes the sequence in slot. The last sequence cannot be
// removed. If it was the current sequence, the lowest remaining slot
// becomes current.
func (p *Project) RemoveSequence(slot int) error {
	i := p.findSequence(slot)
	if i < 0 {
		return fmt.Errorf("no sequence in %s", SlotName(slot))
	}
	if len(p.sequences().Items) == 1 {
		return fmt.Errorf("cannot remove the only sequence")
	}
	// If it is the current sequence, the lowest other slot becomes current.
	next := -1
	if cur := p.data.Lookup("currentSequence"); cur != nil && num(cur) == int64(slot) {
		for _, q := range p.Sequences() {
			if q.Slot != slot {
				next = q.Slot
				break
			}
		}
	}
	// Neither step can fail: i is in range and data is an object.
	_ = p.sequences().Remove(i)
	if next >= 0 {
		_ = p.data.SetKey("currentSequence", xpj.NewInt(int64(next)))
	}
	p.dirty = true
	return nil
}

// songSlot is the key of a song item that names a sequence by its slot
// (checkpoint E).
const songSlot = "item.sequenceIndex"

// sequenceAt returns the sequences[] entry for slot, after checking the slot.
func (p *Project) sequenceAt(slot int) (*xpj.Node, error) {
	if err := checkSeqSlot(slot); err != nil {
		return nil, err
	}
	i := p.findSequence(slot)
	if i < 0 {
		return nil, fmt.Errorf("no sequence in %s", SlotName(slot))
	}
	return p.sequences().Items[i], nil
}

// checkSeqEmpty refuses a slot out of range or holding a sequence.
func (p *Project) checkSeqEmpty(slot int) error {
	if err := checkSeqSlot(slot); err != nil {
		return err
	}
	if p.findSequence(slot) >= 0 {
		return fmt.Errorf("%s already holds a sequence", SlotName(slot))
	}
	return nil
}

// followSlots points song items and currentSequence that name a slot in
// moved at its new slot. Songs and currentSequence name sequences by slot,
// so they keep the same sequences (checkpoint E). Items that are not
// objects or hold no slot number are left alone; fromFile does not check
// songs.
func (p *Project) followSlots(moved map[int64]int64) {
	if songs := p.data.Lookup("songs"); songs != nil {
		for _, song := range songs.Items {
			items := song.Lookup("items")
			if items == nil {
				continue
			}
			for _, it := range items.Items {
				if it.Kind != xpj.Object {
					continue
				}
				if from, err := it.Lookup(songSlot).Int(); err == nil {
					if to, ok := moved[from]; ok {
						_ = it.SetKey(songSlot, xpj.NewInt(to))
					}
				}
			}
		}
	}
	if cur := p.data.Lookup("currentSequence"); cur != nil {
		if from, err := cur.Int(); err == nil {
			if to, ok := moved[from]; ok {
				_ = p.data.SetKey("currentSequence", xpj.NewInt(to))
			}
		}
	}
}

// MoveSequence moves the sequence in from to the empty slot to. Songs and
// the current sequence follow it. Moving a sequence onto its own slot does
// nothing.
func (p *Project) MoveSequence(from, to int) error {
	s, err := p.sequenceAt(from)
	if err != nil {
		return err
	}
	if from == to {
		return nil
	}
	if err := p.checkSeqEmpty(to); err != nil {
		return err
	}
	// fromFile checked that each entry has a whole-number key, so it is an
	// object and these writes cannot fail.
	_ = s.SetKey("key", xpj.NewInt(int64(to)))
	p.followSlots(map[int64]int64{int64(from): int64(to)})
	p.dirty = true
	return nil
}

// SwapSequences exchanges the sequences in slots a and b. Songs and the
// current sequence follow them.
func (p *Project) SwapSequences(a, b int) error {
	sa, err := p.sequenceAt(a)
	if err != nil {
		return err
	}
	sb, err := p.sequenceAt(b)
	if err != nil {
		return err
	}
	if a == b {
		return nil
	}
	_ = sa.SetKey("key", xpj.NewInt(int64(b)))
	_ = sb.SetKey("key", xpj.NewInt(int64(a)))
	p.followSlots(map[int64]int64{int64(a): int64(b), int64(b): int64(a)})
	p.dirty = true
	return nil
}

// CopySequence copies the sequence in from, notes included, to the empty
// slot to. Like the device's own copy, it goes first in sequences[]. Songs
// and the current sequence are unchanged.
func (p *Project) CopySequence(from, to int) error {
	s, err := p.sequenceAt(from)
	if err != nil {
		return err
	}
	if err := p.checkSeqEmpty(to); err != nil {
		return err
	}
	if len(p.sequences().Items) >= MaxSequences {
		return fmt.Errorf("the project already has %d sequences", MaxSequences)
	}
	c := s.Clone()
	_ = c.SetKey("key", xpj.NewInt(int64(to)))
	_ = p.sequences().Insert(0, c)
	p.dirty = true
	return nil
}

// SetSequence sets a sequence's name, BPM and length in bars. Recorded
// notes are left as they are, including any past a new, shorter end.
func (p *Project) SetSequence(slot int, name string, bpm float64, bars int) error {
	i := p.findSequence(slot)
	if i < 0 {
		return fmt.Errorf("no sequence in %s", SlotName(slot))
	}
	if err := ValidName(name); err != nil {
		return fmt.Errorf("sequence name: %w", err)
	}
	// Written as a negated range so that NaN, for which every comparison is
	// false, is refused too.
	if !(bpm >= MinBPM && bpm <= MaxBPM) {
		return fmt.Errorf("BPM must be %g-%g", MinBPM, MaxBPM)
	}
	if bars < 1 || bars > MaxBars {
		return fmt.Errorf("length must be 1-%d bars", MaxBars)
	}
	v := p.sequences().Items[i].Lookup("value")
	clips, err := lengthClips(v)
	if err != nil {
		return err
	}
	// fromFile checked that value is an object, and lengthClips that each clip
	// has a value object, so none of these writes can fail.
	_ = v.SetKey("name", xpj.NewString(name))
	_ = v.SetKey("bpm", xpj.NewFloat(bpm))
	writeLength(v, clips, bars)
	p.lastBPM = bpm
	p.dirty = true
	return nil
}

// setLength writes the 26 fields that hold a sequence's length: four on the
// sequence and two on each of its 11 clips. It writes nothing if the
// sequence does not have that shape.
func setLength(v *xpj.Node, bars int) error {
	clips, err := lengthClips(v)
	if err != nil {
		return err
	}
	writeLength(v, clips, bars)
	return nil
}

// lengthClips returns the value object of each of the sequence's clips,
// checking that every object setLength writes to is there.
func lengthClips(v *xpj.Node) ([]*xpj.Node, error) {
	if v == nil || v.Kind != xpj.Object {
		return nil, errors.New("project: sequence value is not an object")
	}
	clips, err := v.Get("trackClipMaps[0]")
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	var values []*xpj.Node
	for i, c := range clips.Items {
		cv := c.Lookup("value")
		if cv == nil || cv.Kind != xpj.Object {
			return nil, fmt.Errorf("project: clip %d has no value object", i)
		}
		values = append(values, cv)
	}
	return values, nil
}

// writeLength writes a length lengthClips has checked; it cannot fail.
func writeLength(v *xpj.Node, clips []*xpj.Node, bars int) {
	pulses := int64(bars) * PulsesPerBar
	_ = v.SetKey("lengthBars", xpj.NewInt(int64(bars)))
	_ = v.SetKey("loopEndBar", xpj.NewInt(int64(bars)))
	_ = v.SetKey("lengthPulses", xpj.NewInt(pulses))
	_ = v.SetKey("loopEndPulses", xpj.NewInt(pulses))
	for _, cv := range clips {
		_ = cv.SetKey("endPulses", xpj.NewInt(pulses))
		_ = cv.SetKey("loopEndPulses", xpj.NewInt(pulses))
	}
}

// hasNotes reports whether a sequence value holds any note event.
func hasNotes(v *xpj.Node) bool {
	maps := v.Lookup("trackClipMaps")
	if maps == nil {
		return false
	}
	for _, m := range maps.Items {
		for _, c := range m.Items {
			evs, err := c.Get("value.eventList.events")
			if err != nil {
				continue
			}
			for _, ev := range evs.Items {
				if num(ev.Lookup("type")) == eventNote {
					return true
				}
			}
		}
	}
	return false
}

// eachDrumEvent calls fn for every event in every sequence's drum-track
// clip (the clip keyed by tracks[0]'s name) until fn returns false.
func (p *Project) eachDrumEvent(fn func(ev *xpj.Node) bool) {
	track, _ := p.data.Get("tracks[0].name")
	drum := text(track)
	for _, s := range p.sequences().Items {
		maps, err := s.Get("value.trackClipMaps")
		if err != nil {
			continue
		}
		for _, m := range maps.Items {
			for _, c := range m.Items {
				if text(c.Lookup("key")) != drum {
					continue
				}
				evs, err := c.Get("value.eventList.events")
				if err != nil {
					continue
				}
				for _, ev := range evs.Items {
					if !fn(ev) {
						return
					}
				}
			}
		}
	}
}
