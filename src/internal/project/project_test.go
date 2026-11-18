package project

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bajdev/MPCkit/internal/xpj"
)

const testdata = "../../testdata"

func decompress(t *testing.T, path string) []byte {
	t.Helper()
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	zr, err := gzip.NewReader(fh)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func encode(t *testing.T, p *Project) []byte {
	t.Helper()
	b, err := p.File().Encode()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newProject(t *testing.T) *Project {
	t.Helper()
	p, err := New("Test")
	must(t, err)
	return p
}

// leaves flattens a tree into path -> literal text, for comparing files.
func leaves(n *xpj.Node, path string, out map[string]string) map[string]string {
	if out == nil {
		out = map[string]string{}
	}
	switch n.Kind {
	case xpj.Object:
		for _, m := range n.Members {
			leaves(m.Value, path+"."+m.Key, out)
		}
	case xpj.Array:
		for i, it := range n.Items {
			leaves(it, fmt.Sprintf("%s[%d]", path, i), out)
		}
		if len(n.Items) == 0 {
			out[path] = "[]"
		}
	default:
		b, _ := n.MarshalJSON()
		out[path] = string(b)
	}
	return out
}

func diffPaths(a, b *xpj.Node) []string {
	la, lb := leaves(a, "", nil), leaves(b, "", nil)
	var diff []string
	for k, v := range la {
		if lb[k] != v {
			diff = append(diff, k)
		}
	}
	for k := range lb {
		if _, ok := la[k]; !ok {
			diff = append(diff, k)
		}
	}
	return diff
}

// The prototype's edits for MpckitT1 and T3 (build plan appendix), made
// through the model. The output must equal the files the device accepted.
func buildAccepted(t *testing.T, name string, shared bool) *Project {
	t.Helper()
	p, err := New(name)
	must(t, err)
	dir := filepath.Join(testdata, "accepted", name+DataSuffix)
	must(t, p.AssignSample(0, filepath.Join(dir, "Linn 9000 kick.wav")))
	second := "Linn 9000 kick001.wav"
	if shared {
		second = "Linn 9000 kick.wav"
	}
	must(t, p.AssignSample(16, filepath.Join(dir, second)))
	must(t, p.SetTune(16, 5, 0))
	must(t, p.SetSequence(0, "Verse", 96, 8))
	must(t, p.AddSequence(1))
	must(t, p.SetSequence(1, "Chorus", 96, 4))
	return p
}

func TestGoldenMpckitT1(t *testing.T) {
	p := buildAccepted(t, "MpckitT1", false)
	want := decompress(t, filepath.Join(testdata, "accepted", "MpckitT1.xpj"))
	if got := encode(t, p); !bytes.Equal(got, want) {
		f, _ := xpj.Decode(want)
		t.Fatalf("differs from MpckitT1.xpj at %v", diffPaths(p.File().Root, f.Root))
	}
	if p.SampleCount() != 2 {
		t.Errorf("%d samples, want 2", p.SampleCount())
	}
}

func TestGoldenMpckitT3(t *testing.T) {
	p := buildAccepted(t, "MpckitT3", true)
	want := decompress(t, filepath.Join(testdata, "accepted", "MpckitT3.xpj"))
	if got := encode(t, p); !bytes.Equal(got, want) {
		f, _ := xpj.Decode(want)
		t.Fatalf("differs from MpckitT3.xpj at %v", diffPaths(p.File().Root, f.Root))
	}
	if p.SampleCount() != 1 {
		t.Errorf("%d samples, want 1 shared", p.SampleCount())
	}
}

// Rebuild Demo02 from the blank template and compare with the device's file.
func TestRebuildDemo02(t *testing.T) {
	dir := filepath.Join(testdata, "device", "Demo02"+DataSuffix)
	p := newProject(t)
	must(t, p.AssignSample(0, filepath.Join(dir, "Linn 9000 kick.wav")))
	must(t, p.AssignSample(4, filepath.Join(dir, "Linn 9000 kick001.wav")))
	must(t, p.SetTune(4, 12, 40))
	must(t, p.SetTrim(4, 328, 3777))
	must(t, p.SetSequence(0, "Sequence 01", 105, 2))
	must(t, p.AddSequence(1))
	must(t, p.SetSequence(1, "Sequence 02", 105, 4))

	want, err := xpj.ReadFile(filepath.Join(testdata, "device", "Demo02.xpj"))
	must(t, err)
	got, wantLeaves := leaves(p.File().Root, "", nil), leaves(want.Root, "", nil)
	var unexpected []string
	for _, path := range diffPaths(p.File().Root, want.Root) {
		switch {
		case sameFloat(got[path], wantLeaves[path]):
			// The device prints some floats with 17 digits where we print
			// the shortest form: pitch 12.399999618530274 vs ...273 is the
			// same float64.
		case path == ".data.tracks[0].colour",
			path == ".data.assignableEnvelopeFollowerAssignments.currentControlIndex",
			strings.Contains(path, ".eventList.events"):
			// Known noise and recorded notes (build plan M2).
		case path == ".data.tracks[0].program.drum.instruments[4].layersv[0].sliceInfo.LoopCrossfadeLength":
			// The device wrote a different value on the pad it trimmed; we
			// write -1 as for every assigned sample (format doc section 3).
		default:
			unexpected = append(unexpected, path)
		}
	}
	if len(unexpected) > 0 {
		t.Errorf("unexpected differences from Demo02: %v", unexpected)
	}
}

func sameFloat(a, b string) bool {
	fa, errA := strconv.ParseFloat(a, 64)
	fb, errB := strconv.ParseFloat(b, 64)
	return errA == nil && errB == nil && fa == fb
}

// writeWAV writes a short 16-bit mono WAV of frames frames.
func writeWAV(t *testing.T, path string, frames int) string {
	t.Helper()
	data := make([]byte, frames*2)
	var b bytes.Buffer
	le := binary.LittleEndian
	b.WriteString("RIFF")
	binary.Write(&b, le, uint32(4+8+16+8+len(data)))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(44100), uint32(88200), uint16(2), uint16(16)} {
		binary.Write(&b, le, v)
	}
	b.WriteString("data")
	binary.Write(&b, le, uint32(len(data)))
	b.Write(data)
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	must(t, os.WriteFile(path, b.Bytes(), 0o644))
	return path
}

func padOf(t *testing.T, p *Project, slot int) Pad {
	t.Helper()
	v, err := p.Pad(slot)
	must(t, err)
	return v
}

func TestAssignAndPoolReuse(t *testing.T) {
	dir := t.TempDir()
	kick := writeWAV(t, filepath.Join(dir, "kick.wav"), 100)
	p := newProject(t)
	must(t, p.AssignSample(0, kick))
	must(t, p.AssignSample(1, kick))
	if p.SampleCount() != 1 {
		t.Fatalf("same source on two pads: %d pool entries, want 1", p.SampleCount())
	}
	a := padOf(t, p, 1)
	if a.File != "kick.wav" || a.SampleName != "kick" || a.End != 99 {
		t.Errorf("pad A2 = %+v", a)
	}
	if !p.dirty {
		t.Error("not dirty after assign")
	}
	// Reassigning the same source to its only pad keeps the entry.
	must(t, p.ClearPad(1))
	must(t, p.AssignSample(0, kick))
	if p.SampleCount() != 1 || padOf(t, p, 0).File != "kick.wav" {
		t.Errorf("reassign same source: %d entries, pad %+v", p.SampleCount(), padOf(t, p, 0))
	}
	// The source is the file itself, with any links resolved (macOS's
	// temporary folder is behind one).
	want, err := filepath.EvalSymlinks(kick)
	must(t, err)
	if got := p.sources["kick.wav"]; got != want {
		t.Errorf("source = %q, want %q", got, want)
	}
	// Replacing drops the old entry when no pad uses it.
	snare := writeWAV(t, filepath.Join(dir, "snare.wav"), 50)
	must(t, p.AssignSample(0, snare))
	if p.SampleCount() != 1 || p.inPool("kick.wav") || p.sources["kick.wav"] != "" {
		t.Errorf("replace did not drop kick: %d entries", p.SampleCount())
	}
}

func TestNameCollision(t *testing.T) {
	dir := t.TempDir()
	p := newProject(t)
	must(t, p.AssignSample(0, writeWAV(t, filepath.Join(dir, "a", "kick.wav"), 10)))
	must(t, p.AssignSample(1, writeWAV(t, filepath.Join(dir, "b", "kick.wav"), 20)))
	must(t, p.AssignSample(2, writeWAV(t, filepath.Join(dir, "c", "Kick.WAV"), 30)))
	want := []string{"kick.wav", "kick001.wav", "Kick002.WAV"}
	for i, w := range want {
		if got := padOf(t, p, i); got.File != w || got.SampleName != strings.TrimSuffix(w, filepath.Ext(w)) {
			t.Errorf("pad %d = %q/%q, want %q", i, got.SampleName, got.File, w)
		}
	}
	// Both pools hold the same entries; the track pool in reverse order.
	var pool, track []string
	for _, e := range p.pool().Items {
		pool = append(pool, e.Lookup("path").Text)
	}
	for _, e := range p.trackPool().Items {
		track = append(track, e.Lookup("path").Text)
	}
	if strings.Join(pool, ",") != "kick.wav,kick001.wav,Kick002.WAV" || strings.Join(track, ",") != "Kick002.WAV,kick001.wav,kick.wav" {
		t.Errorf("pool %v, track pool %v", pool, track)
	}
}

func TestSampleLimit(t *testing.T) {
	dir := t.TempDir()
	p := newProject(t)
	// With one sample per pad the pool can only fill up when it also holds
	// entries no pad uses, as a loaded project may. Simulate one.
	p.addPoolEntry("unused.wav")
	for i := 0; i < MaxSamples-1; i++ {
		must(t, p.AssignSample(i, writeWAV(t, filepath.Join(dir, fmt.Sprintf("s%03d.wav", i)), 4)))
	}
	if p.SampleCount() != MaxSamples {
		t.Fatalf("%d samples, want %d", p.SampleCount(), MaxSamples)
	}
	err := p.AssignSample(127, writeWAV(t, filepath.Join(dir, "x.wav"), 4))
	if err == nil || !strings.Contains(err.Error(), "128") {
		t.Fatalf("129th sample: %v", err)
	}
	if !padOf(t, p, 127).Empty() || p.SampleCount() != MaxSamples {
		t.Error("refused assign changed the project")
	}
	// Reusing a file already in the pool needs no new entry.
	must(t, p.AssignSample(127, filepath.Join(dir, "s000.wav")))
	// Replacing a pad's only sample frees its entry first.
	must(t, p.AssignSample(1, filepath.Join(dir, "x.wav")))
	if p.SampleCount() != MaxSamples || padOf(t, p, 1).File != "x.wav" {
		t.Errorf("replace at the limit: %d samples, pad %+v", p.SampleCount(), padOf(t, p, 1))
	}
}

func TestAssignRejectsBadFiles(t *testing.T) {
	dir := t.TempDir()
	p := newProject(t)
	bad := filepath.Join(dir, "bad.wav")
	must(t, os.WriteFile(bad, []byte("not a wav"), 0o644))
	if err := p.AssignSample(0, bad); err == nil {
		t.Error("non-WAV accepted")
	}
	if err := p.AssignSample(0, filepath.Join(dir, "missing.wav")); err == nil {
		t.Error("missing file accepted")
	}
	if err := p.AssignSample(128, writeWAV(t, filepath.Join(dir, "ok.wav"), 4)); err == nil {
		t.Error("slot 128 accepted")
	}
	if p.dirty || p.SampleCount() != 0 {
		t.Error("failed assigns changed the project")
	}
}

func TestClearCopyMoveSwap(t *testing.T) {
	dir := t.TempDir()
	p := newProject(t)
	kick := writeWAV(t, filepath.Join(dir, "kick.wav"), 10)
	snare := writeWAV(t, filepath.Join(dir, "snare.wav"), 20)
	must(t, p.AssignSample(0, kick))
	must(t, p.AssignSample(1, snare))
	must(t, p.SetTune(0, 3, -20))
	must(t, p.SetMuteGroup(0, 2))

	// Copy across banks keeps tuning and mute group and shares the entry.
	must(t, p.CopyPad(0, 40))
	c := padOf(t, p, 40)
	if c.File != "kick.wav" || c.Coarse != 3 || c.Fine != -20 || c.MuteGroup != 2 || p.SampleCount() != 2 {
		t.Errorf("copy: %+v, %d samples", c, p.SampleCount())
	}
	// The copy is independent of its source.
	must(t, p.SetTune(40, 0, 0))
	if padOf(t, p, 0).Coarse != 3 {
		t.Error("copy shares data with its source")
	}

	// Move onto a filled pad replaces it; the snare's entry goes.
	must(t, p.MovePad(0, 1))
	if !padOf(t, p, 0).Empty() || padOf(t, p, 1).File != "kick.wav" || p.inPool("snare.wav") {
		t.Errorf("move: A1 %+v, A2 %+v", padOf(t, p, 0), padOf(t, p, 1))
	}
	blank, _ := template.pad.MarshalJSON()
	if got, _ := p.pad(0).MarshalJSON(); !bytes.Equal(got, blank) {
		t.Error("moved-from pad is not the blank pad")
	}

	// Swap.
	must(t, p.AssignSample(5, snare))
	must(t, p.SwapPads(1, 5))
	if padOf(t, p, 1).File != "snare.wav" || padOf(t, p, 5).File != "kick.wav" {
		t.Error("swap")
	}

	// Clearing the last user drops the entry; clearing a shared one keeps it.
	must(t, p.ClearPad(5))
	if !p.inPool("kick.wav") {
		t.Error("cleared a shared entry (kick is still on pad 40)")
	}
	must(t, p.ClearPad(40))
	if p.inPool("kick.wav") || p.SampleCount() != 1 {
		t.Errorf("kick entry kept after its last pad was cleared (%d samples)", p.SampleCount())
	}

	// No-ops and range checks.
	p.dirty = false
	must(t, p.ClearPad(100))
	must(t, p.CopyPad(1, 1))
	must(t, p.MovePad(1, 1))
	must(t, p.SwapPads(1, 1))
	if p.dirty {
		t.Error("no-op marked the project dirty")
	}
	for _, err := range []error{p.CopyPad(-1, 0), p.MovePad(0, 128), p.SwapPads(200, 0), p.ClearPad(128), p.SetTune(128, 0, 0), p.SetMuteGroup(-1, 0)} {
		if err == nil {
			t.Error("out-of-range slot accepted")
		}
	}
}

func TestTuneAndMuteGroup(t *testing.T) {
	p := newProject(t)
	must(t, p.SetTune(0, 12, 40))
	l, _ := layer(p.pad(0))
	if got := l.Lookup("pitch").Text; got != "12.399999618530273" {
		t.Errorf("pitch = %s", got)
	}
	must(t, p.SetTune(0, 99, -500))
	if v := padOf(t, p, 0); v.Coarse != 24 || v.Fine != -90 {
		t.Errorf("clamped tune = %d %d", v.Coarse, v.Fine)
	}
	if got := l.Lookup("pitch").Text; got != "23.100000381469727" {
		t.Errorf("pitch = %s", got)
	}
	must(t, p.SetMuteGroup(3, 1))
	if padOf(t, p, 3).MuteGroup != 1 {
		t.Error("mute group")
	}
	must(t, p.SetMuteGroup(3, 16))
	if err := p.SetMuteGroup(3, MaxMuteGroup+1); err == nil {
		t.Error("mute group out of range accepted")
	}
}

func TestSequences(t *testing.T) {
	p := newProject(t)
	must(t, p.AddSequence(4))
	s := p.Sequences()
	if len(s) != 2 || s[1] != (Sequence{Slot: 4, Name: "Sequence 05", BPM: DefaultBPM, Bars: 2}) {
		t.Fatalf("after add: %+v", s)
	}
	if err := p.AddSequence(4); err == nil {
		t.Error("duplicate slot accepted")
	}
	if err := p.AddSequence(MaxSequences); err == nil {
		t.Error("slot 128 accepted")
	}

	must(t, p.SetSequence(4, "Drop", 140.5, 16))
	must(t, p.AddSequence(127))
	s = p.Sequences()
	if s[1].Name != "Drop" || s[1].BPM != 140.5 || s[1].Bars != 16 || s[2].Name != "Sequence 128" || s[2].BPM != 140.5 {
		t.Errorf("after edit: %+v", s)
	}
	// Newest first in the file, as the device writes.
	if num(p.sequences().Items[0].Lookup("key")) != 127 {
		t.Error("new sequence not inserted first")
	}

	// All 26 length fields.
	v := p.sequences().Items[p.findSequence(4)].Lookup("value")
	n := 0
	for path, val := range leaves(v, "", nil) {
		if strings.HasSuffix(path, "lengthBars") || strings.HasSuffix(path, "loopEndBar") {
			n++
			if val != "16" {
				t.Errorf("%s = %s", path, val)
			}
		}
		if strings.HasSuffix(path, ".lengthPulses") || strings.HasSuffix(path, "loopEndPulses") || strings.HasSuffix(path, ".endPulses") {
			n++
			if val != "61440" {
				t.Errorf("%s = %s", path, val)
			}
		}
	}
	if n != 26 {
		t.Errorf("%d length fields, want 26", n)
	}

	for _, bad := range []struct {
		name string
		bpm  float64
		bars int
	}{{"", 120, 2}, {"a/b", 120, 2}, {"ok", 0, 2}, {"ok", -1, 2}, {"ok", 19.9, 2}, {"ok", 999.5, 2}, {"ok", math.NaN(), 2}, {"ok", math.Inf(1), 2}, {"ok", 120, 0}, {"ok", 120, MaxBars + 1}} {
		if err := p.SetSequence(4, bad.name, bad.bpm, bad.bars); err == nil {
			t.Errorf("SetSequence(%q, %v, %d) accepted", bad.name, bad.bpm, bad.bars)
		}
	}
	must(t, p.SetSequence(4, "Min", MinBPM, 1))
	must(t, p.SetSequence(4, "Max", MaxBPM, MaxBars))
	if err := p.SetSequence(9, "x", 120, 2); err == nil {
		t.Error("missing sequence accepted")
	}

	// Remove: current sequence moves to the lowest remaining slot.
	must(t, p.data.SetKey("currentSequence", xpj.NewInt(4)))
	must(t, p.RemoveSequence(4))
	if cur := num(p.data.Lookup("currentSequence")); cur != 0 {
		t.Errorf("currentSequence = %d, want 0", cur)
	}
	must(t, p.RemoveSequence(0))
	if cur := num(p.data.Lookup("currentSequence")); cur != 127 {
		t.Errorf("currentSequence = %d, want 127", cur)
	}
	if err := p.RemoveSequence(127); err == nil {
		t.Error("removed the last sequence")
	}
	if err := p.RemoveSequence(3); err == nil {
		t.Error("removed a missing sequence")
	}
}

func TestLoadDemoFile(t *testing.T) {
	p, warnings, err := Load(filepath.Join(testdata, "device", "DemoFile.xpj"))
	must(t, err)
	if len(warnings) != 0 {
		t.Errorf("warnings: %v", warnings)
	}
	if p.Name != "DemoFile" || p.dirty || p.SampleCount() != 6 {
		t.Errorf("name %q dirty %v samples %d", p.Name, p.dirty, p.SampleCount())
	}
	for file, src := range p.sources {
		if _, err := os.Stat(src); err != nil {
			t.Errorf("source for %s: %v", file, err)
		}
	}
	// The sequences have notes on A1-A4 (36-39); the other pads have none.
	for slot := 0; slot < Pads; slot++ {
		if got, want := p.PadHasNotes(slot), slot < 4; got != want {
			t.Errorf("PadHasNotes(%s) = %v", SlotName(slot), got)
		}
	}
	s := p.Sequences()
	if len(s) != 2 || s[0].Name != "Intro" || !s[0].HasNotes || s[1].Name != "Sequence 02" || !s[1].HasNotes {
		t.Errorf("sequences %+v", s)
	}
	if v := padOf(t, p, 2); v.MuteGroup != 1 || v.File != "80s-Hat-Dreamer1 SA.wav" {
		t.Errorf("A3 = %+v", v)
	}
	// Loaded and unchanged: writes back byte-identical.
	want := decompress(t, filepath.Join(testdata, "device", "DemoFile.xpj"))
	if !bytes.Equal(encode(t, p), want) {
		t.Error("loaded project does not write back unchanged")
	}

	// Assigning a file already in the project folder reuses its entry.
	must(t, p.AssignSample(50, filepath.Join(testdata, "device", "DemoFile"+DataSuffix, "Bass-Pure C.wav")))
	if p.SampleCount() != 6 {
		t.Errorf("%d samples after reusing a loaded file", p.SampleCount())
	}
}

func TestBlankPadsAndTemplate(t *testing.T) {
	p := newProject(t)
	if !bytes.Equal(encode(t, p), decompress(t, filepath.Join(testdata, "device", "DemoBlank.xpj"))) {
		t.Error("New() is not DemoBlank (is internal/project/template out of date?)")
	}
	for slot := 0; slot < Pads; slot++ {
		if !padOf(t, p, slot).Empty() || p.PadHasNotes(slot) {
			t.Fatalf("pad %d not empty", slot)
		}
	}
}

func TestNames(t *testing.T) {
	p := newProject(t)
	must(t, p.SetName("My Kit 2"))
	if p.Name != "My Kit 2" || !p.dirty {
		t.Error("SetName")
	}
	for _, bad := range []string{"", "  ", "a:b", "a?", "end.", "end ", "CON", "com1.txt", "tab\there"} {
		if err := ValidName(bad); err == nil {
			t.Errorf("ValidName(%q) accepted", bad)
		}
	}
	if _, err := New("bad|name"); err == nil {
		t.Error("New with a bad name accepted")
	}
	for slot, want := range map[int]string{0: "A1", 4: "A5", 16: "B1", 17: "B2", 127: "H16"} {
		if got := SlotName(slot); got != want {
			t.Errorf("SlotName(%d) = %s, want %s", slot, got, want)
		}
	}
}

func TestPoolFileName(t *testing.T) {
	for in, want := range map[string]string{
		"kick.wav":        "kick.wav",
		`kick\one.wav`:    "kick_one.wav",
		"a:b*c?.wav":      "a_b_c_.wav",
		`"q"<>|.wav`:      "_q____.wav",
		"tab\there.wav":   "tab_here.wav",
		"Ünïcode 80s.wav": "Ünïcode 80s.wav",
		"":                "sample.wav",
		".":               "sample.wav",
		"..":              "sample.wav",
		"CON.wav":         "CON_.wav",
		"aux.wav":         "aux_.wav",
		"com1.loop.wav":   "com1_.loop.wav",
		"Lpt9":            "Lpt9_",
		"console.wav":     "console.wav",
		"com10.wav":       "com10.wav",
	} {
		got := poolFileName(in)
		if got != want {
			t.Errorf("poolFileName(%q) = %q, want %q", in, got, want)
		}
		if err := PlainFileName(got); err != nil || reservedOnWindows(got) {
			t.Errorf("poolFileName(%q) = %q: not a safe name", in, got)
		}
	}
}

func TestAssignFollowsLink(t *testing.T) {
	dir := t.TempDir()
	kick := writeWAV(t, filepath.Join(dir, "library", "kick.wav"), 100)
	link := filepath.Join(dir, "favourite.wav")
	if err := os.Symlink(kick, link); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
	p := newProject(t)
	must(t, p.AssignSample(0, link))
	want, err := filepath.EvalSymlinks(kick)
	must(t, err)
	// The pool file is named after the link; it is copied from the target.
	if got := p.PoolFiles(); len(got) != 1 || got[0] != "favourite.wav" || p.sources["favourite.wav"] != want {
		t.Errorf("pool %q, source %q; want favourite.wav from %q", got, p.sources["favourite.wav"], want)
	}
	// The target itself is the same source: no second pool entry.
	must(t, p.AssignSample(1, kick))
	if p.SampleCount() != 1 {
		t.Errorf("%d pool entries, want 1", p.SampleCount())
	}
}

// loadDemo loads a copy of the device's DemoFile, with its data folder,
// after applying change to its data.
func loadDemo(t *testing.T, change func(d *xpj.Node)) *Project {
	t.Helper()
	dir := t.TempDir()
	f, err := xpj.ReadFile(filepath.Join(testdata, "device", "DemoFile.xpj"))
	must(t, err)
	d, err := f.Data()
	must(t, err)
	change(d)
	path := filepath.Join(dir, "DemoFile.xpj")
	must(t, f.WriteFile(path))
	p, _, err := Load(path)
	must(t, err)
	return p
}

// unchanged fails the test if p differs from the encoding and sources
// taken before, or has become dirty.
func unchanged(t *testing.T, p *Project, before []byte, sources map[string]string) {
	t.Helper()
	if !bytes.Equal(encode(t, p), before) {
		t.Error("the project file changed")
	}
	if fmt.Sprint(p.sources) != fmt.Sprint(sources) {
		t.Errorf("sources changed: %v, want %v", p.sources, sources)
	}
	if p.Dirty() {
		t.Error("dirty after a failed edit")
	}
}

func TestFailedEditsChangeNothing(t *testing.T) {
	dir := t.TempDir()
	good := writeWAV(t, filepath.Join(dir, "good.wav"), 100)
	notWAV := filepath.Join(dir, "notes.wav")
	must(t, os.WriteFile(notWAV, []byte("not a WAV file"), 0o644))

	p := loadDemo(t, func(d *xpj.Node) {
		// A2's first layer is not an object, B5 is not an object at all,
		// and a clip of the first sequence has no value.
		must(t, d.Set("tracks[0].program.drum.instruments[1].layersv[0]", xpj.NewInt(7)))
		must(t, d.Set("tracks[0].program.drum.instruments[20]", xpj.NewInt(7)))
		clip, err := d.Get("sequences[0].value.trackClipMaps[0][3]")
		must(t, err)
		clip.DeleteKey("value")
	})
	before := encode(t, p)
	sources := map[string]string{}
	for k, v := range p.sources {
		sources[k] = v
	}
	seq := p.Sequences()[1] // sequences[0] in the file is slot 1

	for _, c := range []struct {
		name string
		edit func() error
	}{
		{"assign a missing file", func() error { return p.AssignSample(0, filepath.Join(dir, "gone.wav")) }},
		{"assign a file that is not a WAV", func() error { return p.AssignSample(0, notWAV) }},
		{"tune a pad whose layer is not an object", func() error { return p.SetTune(1, 3, 0) }},
		{"mute group on a pad that is not an object", func() error { return p.SetMuteGroup(20, 2) }},
		{"sequence whose clip has no value", func() error { return p.SetSequence(seq.Slot, "Renamed", 99, 8) }},
		{"trim a pad whose sample file is missing", func() error { return p.SetTrim(0, 10, 100) }},
		{"rename to a name with ?", func() error { return p.RenameSample(0, "big?kick") }},
		{"bad slot", func() error { return p.SetTune(Pads, 0, 0) }},
		{"clear bank 8", func() error { return p.ClearBank(8) }},
		{"clear bank -1", func() error { return p.ClearBank(-1) }},
		{"remove sequences of bank 8", func() error { return p.RemoveSequenceBank(8) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.edit(); err == nil {
				t.Fatal("no error")
			}
			unchanged(t, p, before, sources)
		})
	}

	// The sample limit: a full pool refuses a new file and changes nothing.
	full := newProject(t)
	for i := range MaxSamples - 1 {
		must(t, full.AssignSample(i, writeWAV(t, filepath.Join(dir, fmt.Sprintf("g%03d.wav", i)), 4)))
	}
	full.addPoolEntry("unused.wav") // a 128th entry no pad uses
	full.MarkExported()
	before = encode(t, full)
	sources = map[string]string{}
	for k, v := range full.sources {
		sources[k] = v
	}
	if err := full.AssignSample(MaxSamples-1, good); err == nil {
		t.Fatal("129th sample accepted")
	}
	unchanged(t, full, before, sources)
}

func TestAssignReplacesBrokenPad(t *testing.T) {
	// AssignSample starts from the blank pad, so a pad with no layers is
	// replaced whole rather than failing halfway.
	p := loadDemo(t, func(d *xpj.Node) {
		must(t, d.Set("tracks[0].program.drum.instruments[5].layersv", xpj.NewArray()))
	})
	kick := writeWAV(t, filepath.Join(t.TempDir(), "kick.wav"), 100)
	must(t, p.AssignSample(5, kick))
	if v := padOf(t, p, 5); v.File != "kick.wav" || v.End != 99 {
		t.Errorf("pad A6 = %+v", v)
	}
}

func TestAssignReusesFreedName(t *testing.T) {
	// Replacing a pad's only sample frees its name for the new file, as
	// before edits were made all-or-nothing.
	dir := t.TempDir()
	p := newProject(t)
	must(t, p.AssignSample(0, writeWAV(t, filepath.Join(dir, "a", "kick.wav"), 10)))
	must(t, p.AssignSample(0, writeWAV(t, filepath.Join(dir, "b", "kick.wav"), 20)))
	if got := p.PoolFiles(); len(got) != 1 || got[0] != "kick.wav" {
		t.Errorf("pool %q, want [kick.wav]", got)
	}
}

// sliceFrames returns a pad's sliceInfo.Start, LoopStart and End, as
// written in the file.
func sliceFrames(t *testing.T, p *Project, slot int) [3]int64 {
	t.Helper()
	l, err := layer(p.pad(slot))
	must(t, err)
	var out [3]int64
	for i, key := range []string{"Start", "LoopStart", "End"} {
		n, err := l.Get("sliceInfo." + key)
		must(t, err)
		v, err := n.Int()
		must(t, err)
		out[i] = v
	}
	return out
}

func TestSetTrim(t *testing.T) {
	dir := t.TempDir()
	kick := writeWAV(t, filepath.Join(dir, "kick.wav"), 1000) // frames 0 to 999
	p := newProject(t)
	must(t, p.AssignSample(0, kick))
	p.MarkExported()

	must(t, p.SetTrim(0, 100, 900))
	if got := sliceFrames(t, p, 0); got != [3]int64{100, 100, 900} {
		t.Errorf("Start, LoopStart, End = %v, want 100 100 900", got)
	}
	if v := padOf(t, p, 0); v.Start != 100 || v.End != 900 || !p.Dirty() {
		t.Errorf("pad %+v, dirty %v", v, p.Dirty())
	}
	// The whole file, and the shortest region, are allowed.
	must(t, p.SetTrim(0, 0, 999))
	must(t, p.SetTrim(0, 998, 999))
}

func TestSetTrimLimits(t *testing.T) {
	dir := t.TempDir()
	kick := writeWAV(t, filepath.Join(dir, "kick.wav"), 1000) // frames 0 to 999
	gone := writeWAV(t, filepath.Join(dir, "gone.wav"), 1000)
	p := newProject(t)
	must(t, p.AssignSample(0, kick))
	must(t, p.AssignSample(1, gone))
	must(t, os.Remove(gone))
	p.MarkExported()
	before := encode(t, p)

	for _, c := range []struct {
		name       string
		slot       int
		start, end int64
		want       string // in the error
	}{
		{"negative start", 0, -1, 500, "start -1 is before the first frame; it can be 0 to 998"},
		{"start equal to end", 0, 500, 500, "start 500 must be before end 500: at least 2 frames must play"},
		{"start after end", 0, 600, 500, "start 600 must be before end 500"},
		{"end past the last frame", 0, 0, 1000, "end 1000 is past the sample's last frame; it can be 1 to 999"},
		{"empty pad", 5, 0, 10, "pad A6 has no sample"},
		{"missing source", 1, 0, 10, "gone.wav: the sample file is missing"},
		{"bad slot", Pads, 0, 10, "out of range"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := p.SetTrim(c.slot, c.start, c.end)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error containing %q", err, c.want)
			}
			if !bytes.Equal(encode(t, p), before) || p.Dirty() {
				t.Error("a refused trim changed the project")
			}
		})
	}
}

func TestTrimTravelsWithPad(t *testing.T) {
	dir := t.TempDir()
	kick := writeWAV(t, filepath.Join(dir, "kick.wav"), 1000)
	snare := writeWAV(t, filepath.Join(dir, "snare.wav"), 600)
	p := newProject(t)
	must(t, p.AssignSample(0, kick))
	must(t, p.SetTrim(0, 100, 900))
	trimmed := [3]int64{100, 100, 900}

	// Copy, move and swap carry the whole pad, start and end included.
	must(t, p.CopyPad(0, 1))
	must(t, p.MovePad(1, 2))
	must(t, p.SwapPads(2, 3))
	for _, slot := range []int{0, 3} {
		if got := sliceFrames(t, p, slot); got != trimmed {
			t.Errorf("pad %s: %v, want %v", SlotName(slot), got, trimmed)
		}
	}
	// Replace builds the pad afresh: the whole new file plays.
	must(t, p.AssignSample(3, snare))
	if got := sliceFrames(t, p, 3); got != [3]int64{0, 0, 599} {
		t.Errorf("replaced pad: %v, want 0 0 599", got)
	}
	if got := sliceFrames(t, p, 0); got != trimmed {
		t.Errorf("pad A1 after the replace: %v, want %v", got, trimmed)
	}
}
