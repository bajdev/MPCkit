package project

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bajdev/MPCkit/internal/xpj"
)

// bankProject has five pads in bank A, A2 and A3 sharing a file and A5
// sharing one with B1.
func bankProject(t *testing.T, dir string) *Project {
	t.Helper()
	files := wavs(t, dir, "a.wav", "b.wav", "c.wav", "d.wav")
	p := newProject(t)
	for _, a := range []struct {
		slot int
		file string
	}{{0, files[0]}, {1, files[1]}, {2, files[1]}, {3, files[2]}, {4, files[3]}, {16, files[3]}} {
		must(t, p.AssignSample(a.slot, a.file))
	}
	p.MarkExported()
	return p
}

func TestClearBank(t *testing.T) {
	dir := t.TempDir()
	p := bankProject(t, dir)
	must(t, p.ClearBank(0))
	for slot := 0; slot < PadsPerBank; slot++ {
		if v := padOf(t, p, slot); v.File != "" {
			t.Errorf("pad %d still holds %q", slot, v.File)
		}
	}
	if v := padOf(t, p, 16); v.File != "d.wav" {
		t.Errorf("B1 holds %q, want d.wav", v.File)
	}
	if got := poolNames(t, p); !slices.Equal(got, [][2]string{{"d", "d.wav"}, {"d", "d.wav"}}) {
		t.Errorf("pool entries %q, want only d.wav in both lists", got)
	}
	if got := fmt.Sprint(p.sources); got != fmt.Sprintf("map[d.wav:%s]", filepath.Join(dir, "d.wav")) {
		t.Errorf("sources %s", got)
	}
	if !p.Dirty() {
		t.Error("not marked changed")
	}

	// The same as clearing each pad in turn.
	q := bankProject(t, dir)
	for slot := 0; slot < PadsPerBank; slot++ {
		must(t, q.ClearPad(slot))
	}
	if !bytes.Equal(encode(t, p), encode(t, q)) || fmt.Sprint(p.sources) != fmt.Sprint(q.sources) {
		t.Error("ClearBank differs from ClearPad on each pad")
	}

	// An empty bank changes nothing.
	p.MarkExported()
	before := encode(t, p)
	must(t, p.ClearBank(0))
	must(t, p.ClearBank(7))
	if !bytes.Equal(encode(t, p), before) || p.Dirty() {
		t.Error("clearing an empty bank changed the project")
	}
}

func TestClearBankKeepsNotes(t *testing.T) {
	p := loadDemo(t, func(*xpj.Node) {})
	seqs := p.Sequences()
	must(t, p.ClearBank(0))
	for slot := 0; slot < 4; slot++ {
		if v := padOf(t, p, slot); v.File != "" || !p.PadHasNotes(slot) {
			t.Errorf("pad %d: file %q, notes %v; want empty with its notes", slot, v.File, p.PadHasNotes(slot))
		}
	}
	if got := p.Sequences(); !slices.Equal(got, seqs) {
		t.Errorf("sequences %+v, want %+v", got, seqs)
	}
	if got := p.PoolFiles(); !slices.Equal(got, []string{"Bass-Pure C.wav", "PadLp-London Cm114 SA.wav"}) {
		t.Errorf("pool files %q", got)
	}
}

func current(t *testing.T, p *Project) int64 {
	t.Helper()
	return num(p.data.Lookup("currentSequence"))
}

func TestRemoveSequenceBank(t *testing.T) {
	// DemoFile has sequences 1 and 2 in bank A; 1 is current.
	build := func() *Project {
		p := loadDemo(t, func(*xpj.Node) {})
		must(t, p.AddSequence(20))
		must(t, p.AddSequence(35))
		p.MarkExported()
		return p
	}
	p := build()
	must(t, p.RemoveSequenceBank(0))
	var slots []int
	for _, q := range p.Sequences() {
		slots = append(slots, q.Slot)
	}
	if !slices.Equal(slots, []int{20, 35}) || current(t, p) != 20 || !p.Dirty() {
		t.Errorf("sequences %v, current %d, dirty %v; want [20 35], 20, true", slots, current(t, p), p.Dirty())
	}

	// The same as removing each sequence in turn.
	q := build()
	must(t, q.RemoveSequence(0))
	must(t, q.RemoveSequence(1))
	if !bytes.Equal(encode(t, p), encode(t, q)) {
		t.Error("RemoveSequenceBank differs from RemoveSequence on each sequence")
	}

	// A bank without sequences changes nothing.
	p.MarkExported()
	before := encode(t, p)
	must(t, p.RemoveSequenceBank(0))
	must(t, p.RemoveSequenceBank(7))
	if !bytes.Equal(encode(t, p), before) || p.Dirty() {
		t.Error("an empty bank changed the project")
	}
}

func TestRemoveSequenceBankKeepsOne(t *testing.T) {
	// Every sequence of DemoFile is in bank A: one fresh one is left in A1.
	p := loadDemo(t, func(*xpj.Node) {})
	must(t, p.RemoveSequenceBank(0))
	got := p.Sequences()
	if len(got) != 1 || got[0].Slot != 0 || got[0].Name != "Sequence 01" || got[0].Bars != 2 || got[0].HasNotes {
		t.Errorf("sequences %+v, want one empty Sequence 01 of 2 bars", got)
	}
	if current(t, p) != 0 || !p.Dirty() {
		t.Errorf("current %d, dirty %v", current(t, p), p.Dirty())
	}

	// The fresh sequence is the one Add sequence makes in the bank's first
	// slot: here B1, in a project whose only sequence is B5.
	p = newProject(t)
	must(t, p.AddSequence(20))
	must(t, p.RemoveSequence(0))
	must(t, p.RemoveSequenceBank(1))
	want := newProject(t)
	must(t, want.AddSequence(16))
	must(t, want.RemoveSequence(0))
	if !bytes.Equal(encode(t, p), encode(t, want)) {
		t.Errorf("fresh sequence differs from AddSequence: %+v", p.Sequences())
	}
	if current(t, p) != 16 {
		t.Errorf("current %d, want 16", current(t, p))
	}
}
