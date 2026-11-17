package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCompareNames(t *testing.T) {
	// Each list is in the order CompareNames must give.
	for _, want := range [][]string{
		{"a.wav", "B.wav", "c.wav"},                           // case is ignored
		{"Bass", "clap", "Snare"},                             // names without numbers
		{"kick 1", "kick 2", "kick 10", "kick 100"},           // numbers as numbers
		{"01", "1", "2", "010"},                               // equal values: plain order
		{"s9 b", "s10 a"},                                     // the number decides first
		{"echo", "Écho", "Edge", "zap"},                       // accents fold to the base letter
		{"Æther", "Apple"},                                    // æ sorts as "ae"
		{"Straße 2", "Strasse 10"},                            // ß sorts as "ss"
		{"x 99999999999999999999", "x 100000000000000000000"}, // longer than any int
		{"kick", "kick 1", "kick a"},                          // a shorter name first
	} {
		got := slices.Clone(want)
		slices.Reverse(got)
		slices.SortStableFunc(got, CompareNames)
		if !slices.Equal(got, want) {
			t.Errorf("sorted %q, want %q", got, want)
		}
	}
}

// wavs writes WAVs with the given names into dir and returns their paths.
func wavs(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	var paths []string
	for _, n := range names {
		paths = append(paths, writeWAV(t, filepath.Join(dir, n), 100))
	}
	return paths
}

func bankFiles(t *testing.T, p *Project, bank int) []string {
	t.Helper()
	var files []string
	for slot := bank * PadsPerBank; slot < (bank+1)*PadsPerBank; slot++ {
		files = append(files, padOf(t, p, slot).File)
	}
	return files
}

func TestFillBankFromFolder(t *testing.T) {
	dir := t.TempDir()
	var names []string
	for i := 20; i >= 1; i-- { // written out of order; s1 ... s20 by name
		names = append(names, fmt.Sprintf("s%d.wav", i))
	}
	wavs(t, dir, names...)
	files, ignored := GatherWAVs([]string{dir})
	if len(files) != 20 || ignored != 0 {
		t.Fatalf("gathered %d files, ignored %d", len(files), ignored)
	}
	p := newProject(t)
	res, err := p.FillBank(2, files) // bank C
	must(t, err)
	if len(res.Slots) != 16 || res.Slots[0] != 32 || res.Slots[15] != 47 || res.LeftOut != 4 || res.Full || len(res.Skipped) != 0 {
		t.Errorf("result %+v", res)
	}
	got := bankFiles(t, p, 2)
	for i, f := range got {
		if want := fmt.Sprintf("s%d.wav", i+1); f != want {
			t.Errorf("C%d holds %q, want %q", i+1, f, want)
		}
	}
	if !p.Dirty() {
		t.Error("not marked changed")
	}
}

func TestFillBankRefusesFilledBank(t *testing.T) {
	dir := t.TempDir()
	files := wavs(t, dir, "a.wav", "b.wav", "c.wav")
	p := newProject(t)
	must(t, p.AssignSample(2*PadsPerBank+7, files[0])) // C8
	p.MarkExported()
	before := encode(t, p)
	_, err := p.FillBank(2, files[1:])
	var notEmpty *BankNotEmptyError
	if !errors.As(err, &notEmpty) || err.Error() != "Bank C is not empty. Drop several files or a folder on an empty bank." {
		t.Fatalf("got %v", err)
	}
	if !bytes.Equal(encode(t, p), before) || p.Dirty() {
		t.Error("a refused fill changed the project")
	}
	if _, err := p.FillBank(8, files); err == nil {
		t.Error("bank 8 accepted")
	}
}

func TestFillBankSkipsDamagedFile(t *testing.T) {
	dir := t.TempDir()
	files := wavs(t, dir, "a.wav", "c.wav")
	bad := filepath.Join(dir, "b.wav")
	must(t, os.WriteFile(bad, []byte("not a WAV"), 0o644))
	gathered, _ := GatherWAVs([]string{dir})
	if !slices.Equal(gathered, []string{files[0], bad, files[1]}) {
		t.Fatalf("gathered %q", gathered)
	}
	p := newProject(t)
	res, err := p.FillBank(3, gathered) // bank D
	must(t, err)
	if !slices.Equal(res.Slots, []int{48, 49}) || len(res.Skipped) != 1 || res.Skipped[0].File != bad ||
		!strings.HasPrefix(res.Skipped[0].Reason, "b.wav:") {
		t.Fatalf("result %+v", res)
	}
	if got := bankFiles(t, p, 3)[:3]; !slices.Equal(got, []string{"a.wav", "c.wav", ""}) {
		t.Errorf("D1-D3 hold %q", got)
	}
}

func TestGatherWAVsFolderContents(t *testing.T) {
	dir := t.TempDir()
	plain := wavs(t, dir, "two.wav", "One.WAV")
	wavs(t, filepath.Join(dir, "sub"), "inside.wav")
	wavs(t, dir, ".hidden.wav")
	must(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, "song.mp3"), []byte("x"), 0o644))
	ignored := 5 // sub, .hidden.wav, notes.txt, song.mp3 and the link
	outside := writeWAV(t, filepath.Join(t.TempDir(), "outside.wav"), 100)
	if err := os.Symlink(outside, filepath.Join(dir, "link.wav")); err != nil {
		ignored-- // links need developer mode on Windows
	}
	files, n := GatherWAVs([]string{dir})
	if !slices.Equal(files, []string{plain[1], plain[0]}) || n != ignored {
		t.Errorf("gathered %q, ignored %d; want One.WAV and two.wav, ignored %d", files, n, ignored)
	}
}

func TestGatherWAVsFilesAndFolderTogether(t *testing.T) {
	loose := wavs(t, t.TempDir(), "b.wav", "d 10.wav")
	dir := t.TempDir()
	inFolder := wavs(t, dir, "a.wav", "c.wav", "d 9.wav")
	files, ignored := GatherWAVs([]string{loose[1], dir, loose[0], filepath.Join(dir, "gone.wav")})
	want := []string{inFolder[0], loose[0], inFolder[1], inFolder[2], loose[1]}
	if !slices.Equal(files, want) || ignored != 1 {
		t.Errorf("gathered %q, ignored %d; want %q, ignored 1", files, ignored, want)
	}
}

func TestFillBankStopsAtSampleLimit(t *testing.T) {
	dir := t.TempDir()
	p := newProject(t)
	must(t, p.AssignSample(0, writeWAV(t, filepath.Join(dir, "first.wav"), 100)))
	for i := 0; p.SampleCount() < MaxSamples-2; i++ {
		p.addPoolEntry(fmt.Sprintf("unused%03d.wav", i)) // 126 samples in all
	}
	res, err := p.FillBank(2, wavs(t, dir, "a.wav", "b.wav", "c.wav", "d.wav", "e.wav"))
	must(t, err)
	if len(res.Slots) != 2 || !res.Full || res.LeftOut != 3 || p.SampleCount() != MaxSamples {
		t.Errorf("result %+v, %d samples", res, p.SampleCount())
	}
}

func TestFillBankSameFileTwice(t *testing.T) {
	kick := writeWAV(t, filepath.Join(t.TempDir(), "kick.wav"), 100)
	files, _ := GatherWAVs([]string{kick, kick})
	p := newProject(t)
	res, err := p.FillBank(1, files)
	must(t, err)
	if !slices.Equal(res.Slots, []int{16, 17}) || p.SampleCount() != 1 {
		t.Errorf("result %+v, %d samples", res, p.SampleCount())
	}
}

func TestFillBankNothingUsable(t *testing.T) {
	text := filepath.Join(t.TempDir(), "notes.txt")
	must(t, os.WriteFile(text, []byte("x"), 0o644))
	files, ignored := GatherWAVs([]string{text})
	if len(files) != 0 || ignored != 1 {
		t.Fatalf("gathered %q, ignored %d", files, ignored)
	}
	p := newProject(t)
	before := encode(t, p)
	res, err := p.FillBank(0, files)
	must(t, err)
	if len(res.Slots) != 0 || !bytes.Equal(encode(t, p), before) || p.Dirty() {
		t.Errorf("result %+v changed the project", res)
	}
}
