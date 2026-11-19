package project

import (
	"bytes"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/bajdev/MPCkit/internal/xpj"
)

// DemoSeqMove (device check D1, docs/checkpoint-e.md): One in A1, Two in A2
// (copied by the device to B5, the first entry), Three in A3 (current), and
// song 1 playing slots 0, 1, 2, 1.
func loadSeqMove(t *testing.T) *Project {
	t.Helper()
	p, _, err := Load(filepath.Join(testdata, "device", "DemoSeqMove.xpj"))
	must(t, err)
	return p
}

func songSlots(t *testing.T, p *Project) []int64 {
	t.Helper()
	items, err := p.data.Get("songs[0].items")
	must(t, err)
	var out []int64
	for _, it := range items.Items {
		out = append(out, num(it.Lookup(songSlot)))
	}
	return out
}

func bySlot(p *Project) map[int]Sequence {
	m := map[int]Sequence{}
	for _, q := range p.Sequences() {
		m[q.Slot] = q
	}
	return m
}

// changedPaths lists the paths that differ between two encodings of a project.
func changedPaths(t *testing.T, before []byte, p *Project) []string {
	t.Helper()
	f, err := xpj.Decode(before)
	must(t, err)
	got := diffPaths(f.Root, p.File().Root)
	sort.Strings(got)
	return got
}

func TestSeqMoveFixture(t *testing.T) {
	p := loadSeqMove(t)
	var slots []int
	for _, q := range p.Sequences() {
		slots = append(slots, q.Slot)
	}
	if !slices.Equal(slots, []int{0, 1, 2, 20}) || current(t, p) != 2 || !slices.Equal(songSlots(t, p), []int64{0, 1, 2, 1}) {
		t.Fatalf("DemoSeqMove: slots %v, current %d, song %v", slots, current(t, p), songSlots(t, p))
	}
}

func TestMoveSequence(t *testing.T) {
	p := loadSeqMove(t)
	before, was := encode(t, p), bySlot(p)
	must(t, p.MoveSequence(1, 40))
	got := bySlot(p)
	moved := was[1]
	moved.Slot = 40
	if _, ok := got[1]; ok || got[40] != moved {
		t.Errorf("A2 %+v, C9 %+v; want C9 %+v", got[1], got[40], moved)
	}
	if s := songSlots(t, p); !slices.Equal(s, []int64{0, 40, 2, 40}) {
		t.Errorf("song %v, want [0 40 2 40]", s)
	}
	// Only the key and the song items change (checkpoint E, Q2).
	want := []string{
		".data.sequences[2].key",
		".data.songs[0].items[1].item.sequenceIndex",
		".data.songs[0].items[3].item.sequenceIndex",
	}
	if c := changedPaths(t, before, p); !slices.Equal(c, want) {
		t.Errorf("changed %q, want %q", c, want)
	}
	if current(t, p) != 2 || !p.Dirty() {
		t.Errorf("current %d, dirty %v", current(t, p), p.Dirty())
	}

	// The current sequence follows a move.
	must(t, p.MoveSequence(2, 50))
	if current(t, p) != 50 {
		t.Errorf("current %d after moving A3 to D3, want 50", current(t, p))
	}
}

func TestSwapSequences(t *testing.T) {
	p := loadSeqMove(t)
	before, was := encode(t, p), bySlot(p)
	must(t, p.SwapSequences(0, 2))
	got := bySlot(p)
	if got[0].Name != "Three" || got[0].Bars != was[2].Bars || got[2].Name != "One" || got[2].Bars != was[0].Bars {
		t.Errorf("A1 %+v, A3 %+v", got[0], got[2])
	}
	if s := songSlots(t, p); !slices.Equal(s, []int64{2, 1, 0, 1}) {
		t.Errorf("song %v, want [2 1 0 1]", s)
	}
	// As checkpoint E's Q3, which the device played correctly.
	want := []string{
		".data.currentSequence",
		".data.sequences[1].key",
		".data.sequences[3].key",
		".data.songs[0].items[0].item.sequenceIndex",
		".data.songs[0].items[2].item.sequenceIndex",
	}
	if c := changedPaths(t, before, p); !slices.Equal(c, want) {
		t.Errorf("changed %q, want %q", c, want)
	}
	if current(t, p) != 0 || !p.Dirty() {
		t.Errorf("current %d (want 0, Three's new slot), dirty %v", current(t, p), p.Dirty())
	}
}

func TestCopySequence(t *testing.T) {
	// The device copied A2 to B5. Without that copy, MPCkit's copy of A2 to
	// B5 gives back the device's file exactly.
	p := loadSeqMove(t)
	want := encode(t, p)
	must(t, p.sequences().Remove(0))
	p.MarkExported()
	must(t, p.CopySequence(1, 20))
	if !bytes.Equal(encode(t, p), want) {
		t.Errorf("copy differs from the device's: %q", changedPaths(t, want, p))
	}
	if !p.Dirty() {
		t.Error("not marked changed")
	}

	// Songs and the current sequence stay; the copy keeps the notes.
	p = loadSeqMove(t)
	was := bySlot(p)
	must(t, p.CopySequence(1, 127))
	got := bySlot(p)
	copied := was[1]
	copied.Slot = 127
	if got[127] != copied || got[1] != was[1] {
		t.Errorf("H16 %+v, A2 %+v", got[127], got[1])
	}
	if !copied.HasNotes {
		t.Error("A2 has no notes; the test wants a sequence with notes")
	}
	if current(t, p) != 2 || !slices.Equal(songSlots(t, p), []int64{0, 1, 2, 1}) {
		t.Errorf("current %d, song %v: a copy changes neither", current(t, p), songSlots(t, p))
	}
	if k := num(p.sequences().Items[0].Lookup("key")); k != 127 {
		t.Errorf("first entry is slot %d, want the copy", k)
	}
}

func TestSequenceMovesRefused(t *testing.T) {
	p := loadSeqMove(t)
	p.MarkExported()
	before := encode(t, p)
	for name, edit := range map[string]func() error{
		"move from an empty slot": func() error { return p.MoveSequence(5, 6) },
		"move onto a filled slot": func() error { return p.MoveSequence(0, 1) },
		"move out of range":       func() error { return p.MoveSequence(0, MaxSequences) },
		"move from out of range":  func() error { return p.MoveSequence(-1, 6) },
		"swap with an empty slot": func() error { return p.SwapSequences(0, 5) },
		"swap out of range":       func() error { return p.SwapSequences(0, 200) },
		"copy from an empty slot": func() error { return p.CopySequence(5, 6) },
		"copy onto a filled slot": func() error { return p.CopySequence(0, 1) },
		"copy out of range":       func() error { return p.CopySequence(0, MaxSequences) },
	} {
		if err := edit(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	// Onto itself: nothing to do.
	must(t, p.MoveSequence(1, 1))
	must(t, p.SwapSequences(1, 1))
	if !bytes.Equal(encode(t, p), before) || p.Dirty() {
		t.Error("a refused or empty edit changed the project")
	}
}

func TestCopyRefusedAtLimit(t *testing.T) {
	// 128 slots hold at most 128 sequences, so the limit check cannot be
	// reached through slots; fill the list with entries for other slots.
	p := loadSeqMove(t)
	seqs := p.sequences()
	for len(seqs.Items) < MaxSequences {
		c := seqs.Items[0].Clone()
		must(t, c.SetKey("key", xpj.NewInt(int64(1000+len(seqs.Items)))))
		seqs.Items = append(seqs.Items, c)
	}
	if err := p.CopySequence(1, 127); err == nil {
		t.Error("copy accepted with 128 sequences")
	}
}

// Sequence errors name slots as the grid does.
func TestSequenceErrorsUseGridLabels(t *testing.T) {
	p := loadSeqMove(t)
	for _, c := range []struct {
		err  error
		want string
	}{
		{p.AddSequence(1), "A2 already holds a sequence"},
		{p.RemoveSequence(5), "no sequence in A6"},
		{p.SetSequence(5, "x", 120, 2), "no sequence in A6"},
		{p.MoveSequence(5, 6), "no sequence in A6"},
		{p.MoveSequence(0, 1), "A2 already holds a sequence"},
		{p.SwapSequences(0, 37), "no sequence in C6"},
		{p.CopySequence(0, 20), "B5 already holds a sequence"},
	} {
		if c.err == nil || c.err.Error() != c.want {
			t.Errorf("error %v, want %q", c.err, c.want)
		}
	}
}
