package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bajdev/MPCkit/internal/project"
)

func TestFillMessage(t *testing.T) {
	slots := func(first, n int) []int {
		var s []int
		for i := range n {
			s = append(s, first+i)
		}
		return s
	}
	skipped := func(n int) []project.Skipped {
		var s []project.Skipped
		for i := range n {
			s = append(s, project.Skipped{Reason: fmt.Sprintf("bad%d.wav: not a WAV file", i+1)})
		}
		return s
	}
	for _, c := range []struct {
		name             string
		bank             int
		res              project.FillResult
		notice, problems string
	}{
		{"bank full, some left out", 2, project.FillResult{Slots: slots(32, 16), LeftOut: 7},
			"Assigned 16 files to C1-C16. 7 more were left out.", ""},
		{"one left out", 2, project.FillResult{Slots: slots(32, 16), LeftOut: 1},
			"Assigned 16 files to C1-C16. 1 more was left out.", ""},
		{"all fit", 3, project.FillResult{Slots: slots(48, 5)},
			"Assigned 5 files to D1-D5.", ""},
		{"one file", 3, project.FillResult{Slots: slots(48, 1)},
			"Assigned 1 file to D1.", ""},
		{"sample limit", 2, project.FillResult{Slots: slots(32, 2), Full: true, LeftOut: 3},
			"Assigned 2 files to C1-C2. The project has 128 samples, the most the device allows: 3 more were left out.", ""},
		{"some unusable", 2, project.FillResult{Slots: slots(32, 3), Skipped: skipped(2)},
			"Assigned 3 files to C1-C3.",
			"2 files could not be used:\nbad1.wav: not a WAV file\nbad2.wav: not a WAV file"},
		{"many unusable", 2, project.FillResult{Slots: slots(32, 1), Skipped: skipped(8)},
			"Assigned 1 file to C1.",
			"8 files could not be used:\nbad1.wav: not a WAV file\nbad2.wav: not a WAV file\nbad3.wav: not a WAV file\n" +
				"bad4.wav: not a WAV file\nbad5.wav: not a WAV file\nand 3 more"},
		{"nothing assigned", 2, project.FillResult{Skipped: skipped(1)},
			"", "No files were assigned to bank C.\n1 file could not be used:\nbad1.wav: not a WAV file"},
		{"nothing assigned, project full", 7, project.FillResult{Full: true, LeftOut: 4},
			"", "No files were assigned to bank H. The project already has 128 samples, the most the device allows."},
	} {
		t.Run(c.name, func(t *testing.T) {
			notice, problems := FillMessage(c.bank, c.res)
			if notice != c.notice || problems != c.problems {
				t.Errorf("got\n  notice %q\n  problems %q\nwant\n  notice %q\n  problems %q", notice, problems, c.notice, c.problems)
			}
		})
	}
}

func TestDropOnBank(t *testing.T) {
	dir := t.TempDir()
	kick, err := os.ReadFile(filepath.Join(kickDir, "Linn 9000 kick.wav"))
	must(t, err)
	for i := 1; i <= 20; i++ {
		must(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("kick %d.wav", i)), kick, 0o644))
	}
	must(t, os.WriteFile(filepath.Join(dir, "broken.wav"), []byte("not a WAV"), 0o644))
	text := filepath.Join(t.TempDir(), "notes.txt")
	must(t, os.WriteFile(text, []byte("x"), 0o644))

	s, _ := newService()
	h := hostOf(s)

	// A folder on empty bank C. broken.wav sorts first: it is skipped and
	// kick 1.wav takes C1. 16 are assigned and 4 left out.
	st, notice, err := h.DropOnBank(2, []string{dir})
	if notice != "Assigned 16 files to C1-C16. 4 more were left out." ||
		err == nil || err.Error() != "1 file could not be used:\nbroken.wav: not a WAV file: too short" {
		t.Errorf("notice %q, error %v", notice, err)
	}
	if st.Pads[32].File != "kick 1.wav" || st.Pads[47].File != "kick 16.wav" || !st.Dirty {
		t.Errorf("C1 %q, C16 %q, dirty %v", st.Pads[32].File, st.Pads[47].File, st.Dirty)
	}

	// The same bank again: refused, nothing changes.
	st, notice, err = h.DropOnBank(2, []string{dir})
	if notice != "" || err == nil || err.Error() != "Bank C is not empty. Drop several files or a folder on an empty bank." {
		t.Errorf("notice %q, error %v", notice, err)
	}
	if st.SampleCount != 16 {
		t.Errorf("%d samples after a refused drop", st.SampleCount)
	}

	// Nothing usable.
	if _, notice, err = h.DropOnBank(3, []string{text}); notice != "" || err == nil || err.Error() != NoWAVs {
		t.Errorf("notice %q, error %v", notice, err)
	}
}
