package export

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/bajdev/MPCkit/internal/project"
	"github.com/bajdev/MPCkit/internal/xpj"
)

const testdata = "../../testdata"

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	must(t, err)
	out, err := io.ReadAll(zr)
	must(t, err)
	return out
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	must(t, err)
	return b
}

// buildT1 makes MpckitT1 through the model (as in the M2 golden test).
func buildT1(t *testing.T) *project.Project {
	t.Helper()
	p, err := project.New("MpckitT1")
	must(t, err)
	dir := filepath.Join(testdata, "accepted", "MpckitT1"+project.DataSuffix)
	must(t, p.AssignSample(0, filepath.Join(dir, "Linn 9000 kick.wav")))
	must(t, p.AssignSample(16, filepath.Join(dir, "Linn 9000 kick001.wav")))
	must(t, p.SetTune(16, 5, 0))
	must(t, p.SetSequence(0, "Verse", 96, 8))
	must(t, p.AddSequence(1))
	must(t, p.SetSequence(1, "Chorus", 96, 4))
	return p
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	must(t, err)
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestFolder(t *testing.T) {
	dest := t.TempDir()
	p := buildT1(t)
	path, err := Folder(p, dest, false)
	must(t, err)
	if path != filepath.Join(dest, "MpckitT1.xpj") {
		t.Errorf("path = %s", path)
	}
	if got := listDir(t, dest); strings.Join(got, "|") != "MpckitT1.xpj|MpckitT1_[ProjectData]" {
		t.Errorf("dest holds %v", got)
	}
	want := gunzip(t, readFile(t, filepath.Join(testdata, "accepted", "MpckitT1.xpj")))
	if !bytes.Equal(gunzip(t, readFile(t, path)), want) {
		t.Error("exported .xpj differs from MpckitT1.xpj")
	}
	src := filepath.Join(testdata, "accepted", "MpckitT1"+project.DataSuffix)
	data := filepath.Join(dest, "MpckitT1"+project.DataSuffix)
	if got := listDir(t, data); strings.Join(got, "|") != "Linn 9000 kick.wav|Linn 9000 kick001.wav" {
		t.Errorf("data folder holds %v", got)
	}
	for _, f := range listDir(t, data) {
		if !bytes.Equal(readFile(t, filepath.Join(data, f)), readFile(t, filepath.Join(src, f))) {
			t.Errorf("%s differs from its source", f)
		}
	}

	// Again without overwrite: refused, nothing touched.
	marker := filepath.Join(data, "keep.txt")
	must(t, os.WriteFile(marker, []byte("x"), 0o644))
	_, err = Folder(p, dest, false)
	var exists *ExistsError
	if !errors.As(err, &exists) || len(exists.Paths) != 2 {
		t.Fatalf("second export: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("refused export changed the folder")
	}
	// With overwrite: the old data folder is replaced, not merged.
	_, err = Folder(p, dest, true)
	must(t, err)
	if _, err := os.Stat(marker); err == nil {
		t.Error("overwrite kept a stale file in the data folder")
	}
	if got := listDir(t, dest); len(got) != 2 {
		t.Errorf("temporary files left behind: %v", got)
	}
}

func TestFolderEmptyProject(t *testing.T) {
	dest := t.TempDir()
	p, err := project.New("Empty")
	must(t, err)
	_, err = Folder(p, dest, false)
	must(t, err)
	// The device creates the data folder even with no samples; so do we.
	if got := listDir(t, filepath.Join(dest, "Empty"+project.DataSuffix)); len(got) != 0 {
		t.Errorf("data folder holds %v", got)
	}
}

func TestRenamedLoadedProject(t *testing.T) {
	p, _, err := project.Load(filepath.Join(testdata, "device", "DemoFile.xpj"))
	must(t, err)
	must(t, p.SetName("Renamed"))
	dest := t.TempDir()
	_, err = Folder(p, dest, false)
	must(t, err)
	got := listDir(t, filepath.Join(dest, "Renamed"+project.DataSuffix))
	if len(got) != 6 {
		t.Errorf("data folder holds %v", got)
	}
	want := gunzip(t, readFile(t, filepath.Join(testdata, "device", "DemoFile.xpj")))
	if !bytes.Equal(gunzip(t, readFile(t, filepath.Join(dest, "Renamed.xpj"))), want) {
		t.Error("renaming changed the file contents")
	}
}

func TestMissingSources(t *testing.T) {
	tmp := t.TempDir()
	p, err := project.New("Missing")
	must(t, err)
	for i, name := range []string{"a.wav", "b.wav", "c.wav"} {
		src := filepath.Join(tmp, name)
		must(t, os.WriteFile(src, readFile(t, filepath.Join(testdata, "accepted", "MpckitT2"+project.DataSuffix, "Linn 9000 kick.wav")), 0o644))
		must(t, p.AssignSample(i, src))
	}
	must(t, os.Remove(filepath.Join(tmp, "a.wav")))
	must(t, os.Remove(filepath.Join(tmp, "c.wav")))

	dest := t.TempDir()
	_, err = Folder(p, dest, false)
	var missing *MissingError
	if !errors.As(err, &missing) || strings.Join(missing.Files, ",") != "a.wav,c.wav" {
		t.Fatalf("Folder: %v", err)
	}
	if !strings.Contains(err.Error(), "2 sample file(s)") {
		t.Errorf("message: %v", err)
	}
	if err := Zip(p, filepath.Join(dest, "m.zip"), false); !errors.As(err, &missing) {
		t.Fatalf("Zip: %v", err)
	}
	if got := listDir(t, dest); len(got) != 0 {
		t.Errorf("failed export wrote %v", got)
	}
}

// readZip returns the entry names of the zip at path, in order, and each
// entry's contents. It closes the zip before returning: Windows cannot
// replace a file that is still open, and TestZip replaces it next.
func readZip(t *testing.T, path string) ([]string, map[string][]byte) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	must(t, err)
	defer zr.Close()
	var names []string
	files := map[string][]byte{}
	for _, f := range zr.File {
		names = append(names, f.Name)
		rc, err := f.Open()
		must(t, err)
		b, err := io.ReadAll(rc)
		rc.Close()
		must(t, err)
		files[f.Name] = b
	}
	return names, files
}

func TestZip(t *testing.T) {
	dest := t.TempDir()
	zpath := filepath.Join(dest, "kit.zip")
	p := buildT1(t)
	must(t, Zip(p, zpath, false))

	names, files := readZip(t, zpath)
	want := "MpckitT1.xpj|MpckitT1_[ProjectData]/|MpckitT1_[ProjectData]/Linn 9000 kick.wav|MpckitT1_[ProjectData]/Linn 9000 kick001.wav"
	if strings.Join(names, "|") != want {
		t.Errorf("entries %v", names)
	}
	if !bytes.Equal(gunzip(t, files["MpckitT1.xpj"]), gunzip(t, readFile(t, filepath.Join(testdata, "accepted", "MpckitT1.xpj")))) {
		t.Error("zipped .xpj differs from MpckitT1.xpj")
	}
	src := filepath.Join(testdata, "accepted", "MpckitT1"+project.DataSuffix, "Linn 9000 kick001.wav")
	if !bytes.Equal(files["MpckitT1_[ProjectData]/Linn 9000 kick001.wav"], readFile(t, src)) {
		t.Error("zipped WAV differs from its source")
	}

	var exists *ExistsError
	if err := Zip(p, zpath, false); !errors.As(err, &exists) {
		t.Errorf("existing zip: %v", err)
	}
	must(t, Zip(p, zpath, true))
	if got := listDir(t, dest); strings.Join(got, "|") != "kit.zip" {
		t.Errorf("temporary files left behind: %v", got)
	}
}

// snapshot returns every file and folder below dir with its contents
// ("<dir>" for a folder).
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			out[rel] = "<dir>"
			return nil
		}
		b, err := os.ReadFile(path)
		out[rel] = string(b)
		return err
	})
	must(t, err)
	return out
}

func sameSnapshot(t *testing.T, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%d entries, want %d: %v", len(got), len(want), keys(got))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s changed or missing", k)
		}
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// symlink makes a link, or skips the test where the system does not allow
// it (Windows without developer mode).
func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
}

func TestSymlinkedSource(t *testing.T) {
	// A copy of MpckitT1 whose data folder holds a link instead of a WAV.
	src := t.TempDir()
	must(t, os.WriteFile(filepath.Join(src, "Kit.xpj"), readFile(t, filepath.Join(testdata, "accepted", "MpckitT1.xpj")), 0o644))
	data := filepath.Join(src, "Kit"+project.DataSuffix)
	must(t, os.Mkdir(data, 0o755))
	acc := filepath.Join(testdata, "accepted", "MpckitT1"+project.DataSuffix)
	must(t, os.WriteFile(filepath.Join(data, "Linn 9000 kick.wav"), readFile(t, filepath.Join(acc, "Linn 9000 kick.wav")), 0o644))
	outside := filepath.Join(t.TempDir(), "secret.txt")
	must(t, os.WriteFile(outside, []byte("secret"), 0o644))
	link := filepath.Join(data, "Linn 9000 kick001.wav")
	symlink(t, outside, link)

	if Usable(link) || !Usable(filepath.Join(data, "Linn 9000 kick.wav")) || Usable(data) || Usable(outside+"x") {
		t.Error("Usable: wrong answer for a link, a file, a folder or a missing file")
	}
	p, _, err := project.Load(filepath.Join(src, "Kit.xpj"))
	must(t, err)
	dest := t.TempDir()
	_, err = Folder(p, dest, false)
	var missing *MissingError
	if !errors.As(err, &missing) || strings.Join(missing.Files, ",") != "Linn 9000 kick001.wav" {
		t.Fatalf("Folder: %v", err)
	}
	if err := Zip(p, filepath.Join(dest, "kit.zip"), false); !errors.As(err, &missing) {
		t.Fatalf("Zip: %v", err)
	}
	if got := listDir(t, dest); len(got) != 0 {
		t.Errorf("failed export wrote %v", got)
	}
}

func TestLinkedDataFolder(t *testing.T) {
	// The whole data folder is a link to a folder holding a file with a
	// pool file's name.
	src := t.TempDir()
	must(t, os.WriteFile(filepath.Join(src, "Kit.xpj"), readFile(t, filepath.Join(testdata, "accepted", "MpckitT1.xpj")), 0o644))
	elsewhere := t.TempDir()
	for _, f := range []string{"Linn 9000 kick.wav", "Linn 9000 kick001.wav"} {
		must(t, os.WriteFile(filepath.Join(elsewhere, f), []byte("private"), 0o644))
	}
	symlink(t, elsewhere, filepath.Join(src, "Kit"+project.DataSuffix))

	p, _, err := project.Load(filepath.Join(src, "Kit.xpj"))
	must(t, err)
	dest := t.TempDir()
	_, err = Folder(p, dest, false)
	var missing *MissingError
	if !errors.As(err, &missing) || len(missing.Files) != 2 {
		t.Fatalf("Folder: %v", err)
	}
	if got := listDir(t, dest); len(got) != 0 {
		t.Errorf("failed export wrote %v", got)
	}
}

func TestBadPoolName(t *testing.T) {
	// Load and AssignSample never produce such a name, and only project can
	// record a source, so the export check is tested directly: both Folder
	// and the zip writer call it before writing anything.
	src := filepath.Join(testdata, "accepted", "MpckitT1"+project.DataSuffix, "Linn 9000 kick.wav")
	items := []item{{name: "../evil.wav", src: src}}
	want := `sample "../evil.wav": file name "../evil.wav" contains a folder separator`
	if err := checkNames(items); err == nil || err.Error() != want {
		t.Errorf("checkNames: got %v, want %q", err, want)
	}
	var b bytes.Buffer
	if err := writeZip(&b, buildT1(t), nil, items); err == nil || err.Error() != want {
		t.Errorf("writeZip: got %v, want %q", err, want)
	}
	if b.Len() != 0 {
		t.Errorf("writeZip wrote %d bytes", b.Len())
	}
	if err := checkNames([]item{{name: "kick.wav", src: src}}); err != nil {
		t.Errorf("checkNames on a plain name: %v", err)
	}
}

func TestMissingDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "gone")
	_, err := Folder(buildT1(t), dest, false)
	if want := "export folder does not exist: " + dest; err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

// failRename makes the listed renames from now fail, counting from 1, until
// the test ends.
func failRename(t *testing.T, fail ...int) {
	t.Helper()
	calls := 0
	rename = func(from, to string) error {
		calls++
		for _, n := range fail {
			if calls == n {
				return &os.LinkError{Op: "rename", Old: from, New: to, Err: errors.New("test failure")}
			}
		}
		return os.Rename(from, to)
	}
	t.Cleanup(func() { rename = os.Rename })
}

func TestOverwriteFailureKeepsOldExport(t *testing.T) {
	// The renames of an overwrite: 1 old data folder aside, 2 old .xpj
	// aside, 3 new data folder in, 4 new .xpj in. Failing any of them
	// leaves the previous export exactly as it was.
	for n := 1; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			dest := t.TempDir()
			_, err := Folder(buildT1(t), dest, false)
			must(t, err)
			before := snapshot(t, dest)

			changed := buildT1(t)
			must(t, changed.ClearPad(16))
			failRename(t, n)
			if _, err := Folder(changed, dest, true); err == nil || !strings.Contains(err.Error(), "test failure") {
				t.Fatalf("Folder: %v", err)
			}
			sameSnapshot(t, snapshot(t, dest), before)
		})
	}
}

func TestOverwriteUndoFails(t *testing.T) {
	dest := t.TempDir()
	_, err := Folder(buildT1(t), dest, false)
	must(t, err)
	before := snapshot(t, dest)

	// The new .xpj cannot be moved in (4), and moving the new data folder
	// back out fails too (5): the old pair stays in the temporary folder.
	failRename(t, 4, 5)
	_, err = Folder(buildT1(t), dest, true)
	if err == nil || !strings.Contains(err.Error(), "could not be put back and is kept in") {
		t.Fatalf("Folder: %v", err)
	}
	_, kept, _ := strings.Cut(err.Error(), "is kept in ")
	if got := snapshot(t, kept); len(got) != len(before) {
		t.Errorf("kept %v, want the %d entries of the old export", keys(got), len(before))
	}
}

func TestOverwriteHalfPair(t *testing.T) {
	// Only the .xpj of an earlier export is there: it is replaced and the
	// data folder created.
	dest := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dest, "MpckitT1.xpj"), []byte("old"), 0o644))
	_, err := Folder(buildT1(t), dest, true)
	must(t, err)
	if got := listDir(t, dest); strings.Join(got, "|") != "MpckitT1.xpj|MpckitT1_[ProjectData]" {
		t.Errorf("dest holds %v", got)
	}
	if string(readFile(t, filepath.Join(dest, "MpckitT1.xpj"))) == "old" {
		t.Error("old .xpj not replaced")
	}
}

func TestBackslashNameExportsAndLoads(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names cannot hold a backslash")
	}
	src := filepath.Join(t.TempDir(), `kick\one.wav`)
	must(t, os.WriteFile(src, readFile(t, filepath.Join(testdata, "accepted", "MpckitT1"+project.DataSuffix, "Linn 9000 kick.wav")), 0o644))
	p, err := project.New("Backslash")
	must(t, err)
	must(t, p.AssignSample(0, src))
	if got := p.PoolFiles(); len(got) != 1 || got[0] != "kick_one.wav" {
		t.Fatalf("pool = %q, want kick_one.wav", got)
	}
	dest := t.TempDir()
	path, err := Folder(p, dest, false)
	must(t, err)
	q, _, err := project.Load(path)
	must(t, err)
	if v, err := q.Pad(0); err != nil || v.File != "kick_one.wav" || v.SampleName != "kick_one" {
		t.Errorf("reloaded pad A1 = %+v, %v", v, err)
	}
}

func TestExportErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	must(t, os.WriteFile(file, nil, 0o644))

	bad := buildT1(t)
	bad.Name = "a/b"
	if _, err := Folder(bad, dir, false); err == nil || !strings.HasPrefix(err.Error(), "project name:") {
		t.Errorf("Folder with a bad name: %v", err)
	}
	if err := Zip(bad, filepath.Join(dir, "kit.zip"), false); err == nil || !strings.HasPrefix(err.Error(), "project name:") {
		t.Errorf("Zip with a bad name: %v", err)
	}

	// A pool file with no recorded source: rename one in the tree.
	unknown := buildT1(t)
	d, err := unknown.File().Data()
	must(t, err)
	must(t, d.Set("samples[0].path", xpj.NewString("other.wav")))
	_, err = Folder(unknown, dir, false)
	if err == nil || !strings.Contains(err.Error(), "other.wav (no source recorded)") {
		t.Errorf("no source: %v", err)
	}

	if _, err := Folder(buildT1(t), file, false); err == nil {
		t.Error("Folder into a file: no error")
	}
	if err := Zip(buildT1(t), filepath.Join(dir, "gone", "kit.zip"), false); err == nil {
		t.Error("Zip into a missing folder: no error")
	}

	_, err = Folder(buildT1(t), dir, false)
	must(t, err)
	_, err = Folder(buildT1(t), dir, false)
	if want := "already exists: " + filepath.Join(dir, "MpckitT1.xpj") + ", " + filepath.Join(dir, "MpckitT1_[ProjectData]"); err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
	if got := listDir(t, dir); strings.Join(got, "|") != "MpckitT1.xpj|MpckitT1_[ProjectData]|file" {
		t.Errorf("failed exports left %v", got)
	}
}

func TestTrimSurvivesExport(t *testing.T) {
	p := buildT1(t)
	must(t, p.SetTrim(16, 100, 2000))
	path, err := Folder(p, t.TempDir(), false)
	must(t, err)
	q, _, err := project.Load(path)
	must(t, err)
	v, err := q.Pad(16)
	must(t, err)
	if v.Start != 100 || v.End != 2000 {
		t.Errorf("reloaded pad B1: start %d, end %d; want 100 and 2000", v.Start, v.End)
	}
	d, err := q.File().Data()
	must(t, err)
	n, err := d.Get("tracks[0].program.drum.instruments[16].layersv[0].sliceInfo.LoopStart")
	must(t, err)
	if loop, _ := n.Int(); loop != 100 {
		t.Errorf("reloaded LoopStart %d, want 100", loop)
	}
}
