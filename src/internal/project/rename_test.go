package project

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// poolNames returns name and path of every entry in both pool lists.
func poolNames(t *testing.T, p *Project) [][2]string {
	t.Helper()
	var out [][2]string
	for _, list := range []string{"samples", "tracks[0].samples"} {
		n, err := p.data.Get(list)
		must(t, err)
		for _, e := range n.Items {
			out = append(out, [2]string{text(e.Lookup("name")), text(e.Lookup("path"))})
		}
	}
	return out
}

func layerNames(t *testing.T, p *Project, slot int) [2]string {
	t.Helper()
	l, err := layer(p.pad(slot))
	must(t, err)
	return [2]string{text(l.Lookup("sampleName")), text(l.Lookup("sampleFile"))}
}

func TestRenameSample(t *testing.T) {
	kick := writeWAV(t, filepath.Join(t.TempDir(), "kick.wav"), 100)
	p := newProject(t)
	must(t, p.AssignSample(0, kick))
	p.MarkExported()

	must(t, p.RenameSample(0, "  Big kick  "))
	if got := poolNames(t, p); !slices.Equal(got, [][2]string{{"Big kick", "Big kick.wav"}, {"Big kick", "Big kick.wav"}}) {
		t.Errorf("pool entries %q", got)
	}
	if got := layerNames(t, p, 0); got != [2]string{"Big kick", "Big kick.wav"} {
		t.Errorf("layer %q", got)
	}
	if v := padOf(t, p, 0); v.File != "Big kick.wav" || v.SampleName != "Big kick" {
		t.Errorf("pad %+v", v)
	}
	if got := p.PoolFiles(); !slices.Equal(got, []string{"Big kick.wav"}) || !p.Dirty() {
		t.Errorf("pool files %q, dirty %v", got, p.Dirty())
	}
	if src, ok := p.Source("Big kick.wav"); !ok || filepath.Base(src) != "kick.wav" {
		t.Errorf("source %q, %v: the file on disk keeps its name", src, ok)
	}

	// The same name again changes nothing.
	p.MarkExported()
	before := encode(t, p)
	must(t, p.RenameSample(0, "Big kick"))
	if !bytes.Equal(encode(t, p), before) || p.Dirty() {
		t.Error("renaming to the same name changed the project")
	}
}

func TestRenameSharedSample(t *testing.T) {
	kick := writeWAV(t, filepath.Join(t.TempDir(), "kick.wav"), 100)
	p := newProject(t)
	must(t, p.AssignSample(0, kick))
	must(t, p.CopyPad(0, 17))
	if users := p.FileUsers()["kick.wav"]; !slices.Equal(users, []int{0, 17}) {
		t.Fatalf("users %v", users)
	}
	must(t, p.RenameSample(17, "Shared")) // from the copy
	for _, slot := range []int{0, 17} {
		if got := layerNames(t, p, slot); got != [2]string{"Shared", "Shared.wav"} {
			t.Errorf("pad %s: %q", SlotName(slot), got)
		}
	}
	if p.SampleCount() != 1 {
		t.Errorf("%d pool entries, want 1", p.SampleCount())
	}
}

func TestRenameSampleRefused(t *testing.T) {
	kick := writeWAV(t, filepath.Join(t.TempDir(), "kick.wav"), 100)
	p := newProject(t)
	must(t, p.AssignSample(0, kick))
	p.MarkExported()
	before := encode(t, p)
	long := strings.Repeat("x", MaxFileName-len(".wav")+1) // 51 with .wav
	for _, c := range []struct {
		name, newName, want string
	}{
		{"empty", "", "name is empty"},
		{"only spaces", "   ", "name is empty"},
		{"question mark", "big?kick", `name contains '?'`},
		{"slash", "a/b", `name contains '/'`},
		{"backslash", `a\b`, `name contains '\\'`},
		{"control character", "a\x01b", "control character"},
		{"ends with a dot", "kick.", "cannot end with a dot"},
		{"Windows device name", "con", "reserved name on Windows"},
		{"too long", long, "is 51 characters; a file name can have at most 50, .wav included"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := p.RenameSample(0, c.newName)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error containing %q", err, c.want)
			}
			if !bytes.Equal(encode(t, p), before) || p.Dirty() {
				t.Error("a refused rename changed the project")
			}
		})
	}
	if err := p.RenameSample(5, "x"); err == nil || err.Error() != "pad A6 has no sample" {
		t.Errorf("empty pad: %v", err)
	}
	// 50 characters with .wav is allowed.
	must(t, p.RenameSample(0, long[1:]))
}

func TestRenameSampleNumbersTakenName(t *testing.T) {
	dir := t.TempDir()
	p := newProject(t)
	for i, n := range []string{"kick.wav", "snare.wav", "hat.wav"} {
		must(t, p.AssignSample(i, writeWAV(t, filepath.Join(dir, n), 100)))
	}
	// Another sample's name, in any case, is numbered as at assign.
	must(t, p.RenameSample(1, "KICK"))
	must(t, p.RenameSample(2, "kick"))
	if got := []string{padOf(t, p, 1).File, padOf(t, p, 2).File}; !slices.Equal(got, []string{"KICK001.wav", "kick002.wav"}) {
		t.Errorf("renamed to %q, want KICK001.wav and kick002.wav", got)
	}
	// A sample may change only the case of its own name, without a number.
	must(t, p.RenameSample(0, "Kick"))
	if got := padOf(t, p, 0).File; got != "Kick.wav" {
		t.Errorf("case-only rename gave %q, want Kick.wav", got)
	}
}

func TestRenameThenAssignSameSource(t *testing.T) {
	kick := writeWAV(t, filepath.Join(t.TempDir(), "kick.wav"), 100)
	p := newProject(t)
	must(t, p.AssignSample(0, kick))
	must(t, p.RenameSample(0, "Big kick"))
	must(t, p.AssignSample(1, kick))
	if p.SampleCount() != 1 || padOf(t, p, 1).File != "Big kick.wav" {
		t.Errorf("%d pool entries, A2 holds %q; want 1 and Big kick.wav", p.SampleCount(), padOf(t, p, 1).File)
	}
}
