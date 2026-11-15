package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"git.monoexp.com/bajdev/mpckit/internal/xpj"
)

// loadChanged reads the device's DemoFile, applies change to its data, writes
// it to a temporary folder and loads it.
func loadChanged(t *testing.T, change func(d *xpj.Node)) error {
	t.Helper()
	f, err := xpj.ReadFile(filepath.Join(testdata, "device", "DemoFile.xpj"))
	must(t, err)
	d, err := f.Data()
	must(t, err)
	change(d)
	path := filepath.Join(t.TempDir(), "Changed.xpj")
	must(t, f.WriteFile(path))
	_, _, err = Load(path)
	return err
}

func set(t *testing.T, d *xpj.Node, path string, v *xpj.Node) {
	t.Helper()
	must(t, d.Set(path, v))
}

func remove(t *testing.T, d *xpj.Node, path, key string) {
	t.Helper()
	n, err := d.Get(path)
	must(t, err)
	if !n.DeleteKey(key) {
		t.Fatalf("%s has no key %q", path, key)
	}
}

func TestLoadRejectsMalformed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, d *xpj.Node)
		want   string // in the error
	}{
		{"parent folder", func(t *testing.T, d *xpj.Node) {
			set(t, d, "samples[0].path", xpj.NewString("../x.wav"))
		}, `samples[0].path: file name "../x.wav" contains a folder separator`},
		{"subfolder", func(t *testing.T, d *xpj.Node) {
			set(t, d, "samples[1].path", xpj.NewString("a/b.wav"))
		}, `samples[1].path: file name "a/b.wav" contains a folder separator`},
		{"backslash", func(t *testing.T, d *xpj.Node) {
			set(t, d, "samples[1].path", xpj.NewString(`..\b.wav`))
		}, `samples[1].path: file name "..\\b.wav" contains a folder separator`},
		{"dot dot", func(t *testing.T, d *xpj.Node) {
			set(t, d, "samples[2].path", xpj.NewString(".."))
		}, `samples[2].path: ".." is not a file name`},
		{"empty", func(t *testing.T, d *xpj.Node) {
			set(t, d, "samples[0].path", xpj.NewString(""))
		}, `samples[0].path: file name is empty`},
		{"control character", func(t *testing.T, d *xpj.Node) {
			set(t, d, "samples[0].path", xpj.NewString("a\x00.wav"))
		}, `samples[0].path: file name "a\x00.wav" contains a control character`},
		{"track pool", func(t *testing.T, d *xpj.Node) {
			set(t, d, "tracks[0].samples[0].path", xpj.NewString("../x.wav"))
		}, `tracks[0].samples[0].path: file name "../x.wav"`},
		{"missing path", func(t *testing.T, d *xpj.Node) {
			remove(t, d, "samples[3]", "path")
		}, "samples[3].path is missing or not text"},
		{"path not text", func(t *testing.T, d *xpj.Node) {
			set(t, d, "samples[3].path", xpj.NewInt(3))
		}, "samples[3].path is missing or not text"},
		{"layer outside", func(t *testing.T, d *xpj.Node) {
			set(t, d, "tracks[0].program.drum.instruments[17].layersv[2].sampleFile", xpj.NewString("../x.wav"))
		}, `pad B2 layer 3 sampleFile: file name "../x.wav" contains a folder separator`},
		{"layer not text", func(t *testing.T, d *xpj.Node) {
			set(t, d, "tracks[0].program.drum.instruments[0].layersv[0].sampleFile", xpj.NewNull())
		}, "pad A1 layer 1 sampleFile is not text"},
		{"missing bpm", func(t *testing.T, d *xpj.Node) {
			remove(t, d, "sequences[0].value", "bpm")
		}, "sequences[0]: bpm is missing or not a number"},
		{"missing key", func(t *testing.T, d *xpj.Node) {
			remove(t, d, "sequences[1]", "key")
		}, "sequences[1]: key is missing or not a whole number"},
		{"missing value", func(t *testing.T, d *xpj.Node) {
			set(t, d, "sequences[1].value", xpj.NewArray())
		}, "sequences[1]: value is missing or not an object"},
		{"missing length", func(t *testing.T, d *xpj.Node) {
			remove(t, d, "sequences[0].value", "lengthBars")
		}, "sequences[0]: lengthBars is missing or not a whole number"},
		{"name not text", func(t *testing.T, d *xpj.Node) {
			set(t, d, "sequences[0].value.name", xpj.NewInt(1))
		}, "sequences[0]: name is missing or not text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := loadChanged(t, func(d *xpj.Node) { tc.change(t, d) })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got error %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestLoadAcceptsMissingLayerFile(t *testing.T) {
	err := loadChanged(t, func(d *xpj.Node) {
		remove(t, d, "tracks[0].program.drum.instruments[0].layersv[0]", "sampleFile")
	})
	must(t, err)
}

func TestLoadMissingVersion(t *testing.T) {
	f, err := xpj.ReadFile(filepath.Join(testdata, "device", "DemoFile.xpj"))
	must(t, err)
	d, err := f.Data()
	must(t, err)
	d.DeleteKey("version")
	path := filepath.Join(t.TempDir(), "NoVersion.xpj")
	must(t, f.WriteFile(path))
	_, warnings, err := Load(path)
	must(t, err)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "version is missing") {
		t.Fatalf("warnings = %q", warnings)
	}
}

func TestPlainFileName(t *testing.T) {
	for _, ok := range []string{"kick.wav", "80s-Kik-Dreamer2 SA.wav", "a..b.wav", ".hidden.wav", "Ünïcode.wav"} {
		if err := PlainFileName(ok); err != nil {
			t.Errorf("PlainFileName(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".", "..", "../x.wav", "a/b.wav", `a\b.wav`, "/x.wav", "x\n.wav", "x\x7f.wav"} {
		if err := PlainFileName(bad); err == nil {
			t.Errorf("PlainFileName(%q) = nil", bad)
		}
	}
}

// FuzzLoad loads arbitrary files; none may make the read-only calls panic.
func FuzzLoad(f *testing.F) {
	seeds, err := filepath.Glob(filepath.Join(testdata, "device", "*.xpj"))
	if err != nil || len(seeds) == 0 {
		f.Fatalf("no seed files: %v", err)
	}
	for _, s := range seeds {
		b, err := os.ReadFile(s)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		path := filepath.Join(t.TempDir(), "Fuzz.xpj")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		p, _, err := Load(path)
		if err != nil {
			return
		}
		for slot := range Pads {
			_, _ = p.Pad(slot)
		}
		p.Sequences()
		p.PoolFiles()
	})
}

// TestBrokenFieldsDoNotPanic changes, one at a time, each field that project
// reads in DemoFile: replaced by null, text, a negative or large number, an
// empty list or object, or deleted. Whatever fromFile makes of the result, reads
// and edits must not panic. Only the first item of each list is changed, so
// the test stays fast; a full sweep (all fields to depth 5, 12,726 cases)
// found nothing either and takes about 10 minutes.
func TestBrokenFieldsDoNotPanic(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	base, err := xpj.ReadFile(filepath.Join(testdata, "device", "DemoFile.xpj"))
	must(t, err)
	d, err := base.Data()
	must(t, err)

	events := "sequences[0].value.trackClipMaps[0][0].value.eventList.events"
	roots := []struct {
		path  string
		depth int
	}{
		{"version", 0}, {"currentSequence", 0}, {"tracks[0].name", 0},
		{"samples", 3}, {"tracks[0].samples", 2},
		{"tracks[0].program.drum.instruments", 1},
		{"tracks[0].program.drum.instruments[0]", 1},
		{"tracks[0].program.drum.instruments[0].layersv", 2},
		{"tracks[0].program.drum.instruments[0].layersv[0].sliceInfo", 1},
		{"sequences", 3},
		{"sequences[0].value.trackClipMaps", 4},
		{events, 3},
	}
	// The first event is automation; add the first note event too.
	evs, err := d.Get(events)
	must(t, err)
	for i, ev := range evs.Items {
		if num(ev.Lookup("type")) == eventNote {
			roots = append(roots, struct {
				path  string
				depth int
			}{fmt.Sprintf("%s[%d]", events, i), 1}, struct {
				path  string
				depth int
			}{fmt.Sprintf("%s[%d].note.note", events, i), 0})
			break
		}
	}
	var paths []string
	for _, r := range roots {
		n, err := d.Get(r.path)
		must(t, err)
		fieldPaths(n, r.path, r.depth, &paths)
	}

	changes := []func(parent *xpj.Node, key string, i int){
		func(p *xpj.Node, k string, i int) { replace(p, k, i, xpj.NewNull()) },
		func(p *xpj.Node, k string, i int) { replace(p, k, i, xpj.NewString("../x.wav")) },
		func(p *xpj.Node, k string, i int) { replace(p, k, i, xpj.NewInt(-1)) },
		func(p *xpj.Node, k string, i int) { replace(p, k, i, xpj.NewInt(1000)) },
		func(p *xpj.Node, k string, i int) { replace(p, k, i, xpj.NewArray()) },
		func(p *xpj.Node, k string, i int) { replace(p, k, i, xpj.NewObject()) },
		func(p *xpj.Node, k string, i int) {
			if k != "" {
				p.DeleteKey(k)
			} else {
				_ = p.Remove(i)
			}
		},
	}
	names := []string{"null", "text", "-1", "1000", "list", "object", "deleted"}
	for _, path := range paths {
		parentPath, key, i := splitPath(path)
		for c, change := range changes {
			f := base.Clone()
			fd, _ := f.Data()
			parent, err := fd.Get(parentPath)
			if err != nil {
				continue // a key holding '.' or '[' cannot be addressed by path
			}
			change(parent, key, i)
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s %s: panic: %v", path, names[c], r)
					}
				}()
				p, err := fromFile(f, "Broken", "")
				if err != nil {
					return
				}
				exercise(p)
			}()
		}
	}
	t.Logf("%d fields, %d cases", len(paths), len(paths)*len(changes))
}

// fieldPaths lists path and the paths below n to depth levels, taking only
// the first item of each list.
func fieldPaths(n *xpj.Node, path string, depth int, out *[]string) {
	*out = append(*out, path)
	if depth == 0 {
		return
	}
	if len(n.Items) > 0 {
		fieldPaths(n.Items[0], path+"[0]", depth-1, out)
	}
	for _, m := range n.Members {
		fieldPaths(m.Value, path+"."+m.Key, depth-1, out)
	}
}

// splitPath splits a path into its parent and last step: a key, or an index
// when key is "".
func splitPath(path string) (parent, key string, i int) {
	if strings.HasSuffix(path, "]") {
		open := strings.LastIndexByte(path, '[')
		i, _ = strconv.Atoi(path[open+1 : len(path)-1])
		return path[:open], "", i
	}
	dot := strings.LastIndexByte(path, '.')
	if dot < 0 {
		return "", path, 0
	}
	return path[:dot], path[dot+1:], 0
}

func replace(parent *xpj.Node, key string, i int, v *xpj.Node) {
	if key != "" {
		_ = parent.SetKey(key, v)
	} else if i < len(parent.Items) {
		parent.Items[i] = v
	}
}

// exercise runs every read and most edits on p.
func exercise(p *Project) {
	for slot := range Pads {
		_, _ = p.Pad(slot)
		p.PadHasNotes(slot)
	}
	p.PoolFiles()
	for slot := range 2 {
		_ = p.SetTune(slot, 1, 1)
		_ = p.SetMuteGroup(slot, 1)
		_ = p.CopyPad(slot, slot+20)
		_ = p.MovePad(slot+20, slot+40)
		_ = p.SwapPads(slot, slot+1)
		_ = p.ClearPad(slot + 1)
	}
	for _, s := range p.Sequences() {
		_ = p.SetSequence(s.Slot, "Changed", 100, 4)
	}
	_ = p.AddSequence(50)
	_ = p.RemoveSequence(50)
	_ = p.RemoveSequence(0)
}
