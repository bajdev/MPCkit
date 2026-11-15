package app

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"git.monoexp.com/bajdev/mpckit/internal/project"
)

const testdata = "../../testdata"

// fakeDialogs returns preset answers; "" is a cancelled dialog.
type fakeDialogs struct {
	wav, project, folder, zip string
	zipDefault                string
}

func (f *fakeDialogs) OpenWAV() (string, error)      { return f.wav, nil }
func (f *fakeDialogs) OpenProject() (string, error)  { return f.project, nil }
func (f *fakeDialogs) ChooseFolder() (string, error) { return f.folder, nil }
func (f *fakeDialogs) SaveZip(name string) (string, error) {
	f.zipDefault = name
	return f.zip, nil
}

func newService() (*Service, *fakeDialogs) {
	d := &fakeDialogs{}
	s, _ := NewService(d)
	return s, d
}

// hostOf returns the Host that shares s, as NewService does.
func hostOf(s *Service) *Host { return &Host{s: s} }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

var kickDir = filepath.Join(testdata, "accepted", "MpckitT1"+project.DataSuffix)

func TestNewState(t *testing.T) {
	s, _ := newService()
	st := s.State()
	if st.Name != DefaultName || st.Dirty || st.SampleCount != 0 {
		t.Errorf("state %q dirty %v samples %d", st.Name, st.Dirty, st.SampleCount)
	}
	if len(st.Pads) != 128 || st.Pads[0].Label != "A1" || st.Pads[17].Label != "B2" || st.Pads[127].Label != "H16" {
		t.Fatalf("pads: %d", len(st.Pads))
	}
	for _, p := range st.Pads {
		if p.File != "" || p.HasNotes || p.Audio != "" {
			t.Fatalf("pad %s not empty", p.Label)
		}
	}
	if st.SequenceCount != 1 || st.Sequences[0] != (Sequence{Slot: 0, Label: "A1", Name: "Sequence 01", BPM: 128, Bars: 2}) {
		t.Errorf("sequences %+v", st.Sequences)
	}
	if st.Limits.MaxMuteGroup != 16 || st.Limits.MinBPM != 20 || st.Limits.MaxBPM != 999 || st.Limits.MinCoarse != -24 {
		t.Errorf("limits %+v", st.Limits)
	}
	if st2 := s.New(); st2.Name != DefaultName || len(st2.Pads) != 128 {
		t.Error("New")
	}
}

func TestStateJSON(t *testing.T) {
	s, _ := newService()
	b, err := json.Marshal(s.State())
	must(t, err)
	for _, key := range []string{`"name":"NewKit"`, `"pads":[{"slot":0,"label":"A1"`, `"sequences":[{"slot":0`, `"limits":{`, `"muteGroup":0`, `"audio":""`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("JSON lacks %s", key)
		}
	}
}

func TestAbout(t *testing.T) {
	s, _ := newService()
	a := s.About()
	if a.Version != Version || !strings.Contains(a.License, "2026 bajdev") || !strings.HasPrefix(a.Repo, "https://") {
		t.Errorf("%+v", a)
	}
}

func TestPadEdits(t *testing.T) {
	s, d := newService()
	kick := filepath.Join(kickDir, "Linn 9000 kick.wav")

	// Cancelled dialog: nothing changes, no error.
	st, err := s.AssignSampleDialog(0)
	must(t, err)
	if st.Pads[0].File != "" || st.Dirty {
		t.Error("cancelled dialog changed the project")
	}

	d.wav = kick
	st, err = s.AssignSampleDialog(0)
	must(t, err)
	p := st.Pads[0]
	if p.File != "Linn 9000 kick.wav" || p.Name != "Linn 9000 kick" || p.End != 8212 || p.Start != 0 || !strings.HasPrefix(p.Audio, "/audio/0?k=") || !st.Dirty || st.SampleCount != 1 {
		t.Errorf("after assign: %+v", p)
	}

	st, err = hostOf(s).AssignSamplePath(16, kick)
	must(t, err)
	if st.Pads[16].File != "Linn 9000 kick.wav" || st.SampleCount != 1 {
		t.Error("drop of the same file did not share the pool entry")
	}
	if st.Pads[16].Audio == st.Pads[0].Audio || !strings.HasSuffix(st.Pads[16].Audio, st.Pads[0].Audio[strings.Index(st.Pads[0].Audio, "?"):]) {
		t.Errorf("audio URLs %q %q: want same key, different slot", st.Pads[0].Audio, st.Pads[16].Audio)
	}

	st, err = s.SetTune(16, 5, -20)
	must(t, err)
	if st.Pads[16].Coarse != 5 || st.Pads[16].Fine != -20 {
		t.Error("tune")
	}
	st, err = s.SetMuteGroup(16, 3)
	must(t, err)
	if st.Pads[16].MuteGroup != 3 {
		t.Error("mute group")
	}
	st, err = s.SetMuteGroup(16, 17)
	if err == nil || st.Pads[16].MuteGroup != 3 {
		t.Error("mute group 17 accepted, or state lost on error")
	}

	st, err = s.ClearPad(0)
	must(t, err)
	if st.Pads[0].File != "" || st.SampleCount != 1 {
		t.Error("clear")
	}

	// Errors still return the current state.
	st, err = hostOf(s).AssignSamplePath(1, filepath.Join(testdata, "nope.wav"))
	if err == nil || st.Pads[16].File == "" {
		t.Errorf("bad path: %v", err)
	}
}

func TestSetName(t *testing.T) {
	s, _ := newService()
	st, err := s.SetName("  My Kit ")
	must(t, err)
	if st.Name != "My Kit" || !st.Dirty {
		t.Errorf("name %q", st.Name)
	}
	st, err = s.SetName("bad/name")
	if err == nil || st.Name != "My Kit" {
		t.Errorf("bad name: %v, name now %q", err, st.Name)
	}
}

func gunzip(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	must(t, err)
	defer f.Close()
	zr, err := gzip.NewReader(f)
	must(t, err)
	b, err := io.ReadAll(zr)
	must(t, err)
	return b
}

// The M5 done-when in miniature: the UI's calls build the same file as the
// prototype and the CLI (MpckitT1 without its sequences).
func TestExport(t *testing.T) {
	s, d := newService()
	_, err := s.SetName("Kit")
	must(t, err)
	_, err = hostOf(s).AssignSamplePath(0, filepath.Join(kickDir, "Linn 9000 kick.wav"))
	must(t, err)

	d.folder = t.TempDir()
	dir, err := s.ChooseExportFolder()
	must(t, err)
	res, err := s.ExportFolder(false)
	must(t, err)
	if res.Path != filepath.Join(dir, "Kit.xpj") || len(res.Exists) != 0 {
		t.Fatalf("export: %+v", res)
	}
	if s.State().Dirty {
		t.Error("still dirty after export")
	}
	if _, err := os.Stat(filepath.Join(dir, "Kit"+project.DataSuffix, "Linn 9000 kick.wav")); err != nil {
		t.Error(err)
	}

	// The target is forgotten after a successful export.
	if _, err := s.ExportFolder(false); err == nil || err.Error() != "choose where to export first" {
		t.Errorf("export without choosing again: %v", err)
	}
	// Second time: reported, not written, until overwrite. The confirmed
	// call uses the same target.
	_, err = s.ChooseExportFolder()
	must(t, err)
	res, err = s.ExportFolder(false)
	must(t, err)
	if res.Path != "" || len(res.Exists) != 2 {
		t.Errorf("re-export: %+v", res)
	}
	res, err = s.ExportFolder(true)
	must(t, err)
	if res.Path == "" {
		t.Error("overwrite export failed")
	}

	d.zip = filepath.Join(dir, "kit-export") // no extension: .zip is added
	zpath, err := s.ChooseZipPath()
	must(t, err)
	if d.zipDefault != "Kit.zip" || zpath != filepath.Join(dir, "kit-export.zip") {
		t.Errorf("zip dialog default %q, chosen %q", d.zipDefault, zpath)
	}
	res, err = s.ExportZip(false)
	must(t, err)
	if res.Path != filepath.Join(dir, "kit-export.zip") {
		t.Errorf("zip: %+v", res)
	}
	_, err = s.ChooseZipPath()
	must(t, err)
	if res, _ = s.ExportZip(false); len(res.Exists) != 1 {
		t.Errorf("zip re-export: %+v", res)
	}

	// Missing folder: a real error.
	d.folder = filepath.Join(dir, "missing", "deeper")
	_, err = s.ChooseExportFolder()
	must(t, err)
	if _, err := s.ExportFolder(false); err == nil {
		t.Error("export into a missing folder succeeded")
	}
}

func TestExportTarget(t *testing.T) {
	s, d := newService()
	for _, export := range []func(bool) (ExportResult, error){s.ExportFolder, s.ExportZip} {
		if _, err := export(false); err == nil || err.Error() != "choose where to export first" {
			t.Errorf("export with nothing chosen: %v", err)
		}
	}

	// A cancelled dialog forgets an earlier choice.
	d.folder, d.zip = t.TempDir(), filepath.Join(t.TempDir(), "kit.zip")
	_, err := s.ChooseExportFolder()
	must(t, err)
	_, err = s.ChooseZipPath()
	must(t, err)
	d.folder, d.zip = "", ""
	_, err = s.ChooseExportFolder()
	must(t, err)
	_, err = s.ChooseZipPath()
	must(t, err)
	if s.exportDir != "" || s.zipPath != "" {
		t.Errorf("cancel kept %q, %q", s.exportDir, s.zipPath)
	}

	// New and Load forget the targets chosen for the old project.
	d.folder, d.zip = t.TempDir(), filepath.Join(t.TempDir(), "kit.zip")
	choose := func() {
		_, err := s.ChooseExportFolder()
		must(t, err)
		_, err = s.ChooseZipPath()
		must(t, err)
	}
	choose()
	s.New()
	if s.exportDir != "" || s.zipPath != "" {
		t.Error("New kept the export targets")
	}
	choose()
	_, err = hostOf(s).LoadPath(filepath.Join(testdata, "device", "DemoBlank.xpj"))
	must(t, err)
	if s.exportDir != "" || s.zipPath != "" {
		t.Error("Load kept the export targets")
	}
}

// exportTo chooses dir in the fake dialog and exports there.
func exportTo(t *testing.T, s *Service, dir string) (ExportResult, error) {
	t.Helper()
	s.dialogs.(*fakeDialogs).folder = dir
	_, err := s.ChooseExportFolder()
	must(t, err)
	return s.ExportFolder(false)
}

func TestExportMatchesCLIRecipe(t *testing.T) {
	// Same edits as MpckitT1's pads, through the service.
	s, _ := newService()
	_, err := s.SetName("MpckitT1")
	must(t, err)
	_, err = hostOf(s).AssignSamplePath(0, filepath.Join(kickDir, "Linn 9000 kick.wav"))
	must(t, err)
	_, err = hostOf(s).AssignSamplePath(16, filepath.Join(kickDir, "Linn 9000 kick001.wav"))
	must(t, err)
	_, err = s.SetTune(16, 5, 0)
	must(t, err)
	dir := t.TempDir()
	res, err := exportTo(t, s, dir)
	must(t, err)

	p, err := project.New("MpckitT1")
	must(t, err)
	must(t, p.AssignSample(0, filepath.Join(kickDir, "Linn 9000 kick.wav")))
	must(t, p.AssignSample(16, filepath.Join(kickDir, "Linn 9000 kick001.wav")))
	must(t, p.SetTune(16, 5, 0))
	want, err := p.File().Encode()
	must(t, err)
	if !bytes.Equal(gunzip(t, res.Path), want) {
		t.Error("service export differs from the model's")
	}
}

func TestAudio(t *testing.T) {
	s, _ := newService()
	kick := filepath.Join(kickDir, "Linn 9000 kick.wav")
	st, err := hostOf(s).AssignSamplePath(3, kick)
	must(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("asset")) })
	h := hostOf(s).AudioMiddleware(next)
	get := func(url string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		return rec
	}

	if rec := get("/index.html"); rec.Body.String() != "asset" {
		t.Error("other paths are not passed on")
	}
	rec := get(st.Pads[3].Audio)
	want, _ := os.ReadFile(kick)
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), want) {
		t.Errorf("wav: %d, %d bytes", rec.Code, rec.Body.Len())
	}

	// The plain answer gives the file's own length and rate, so the preview
	// can place start and end whatever rate the webview decodes to.
	slow := filepath.Join(testdata, "wav-formats", "MpckitW1_[ProjectData]", "22k 16bit mono.wav")
	_, err = hostOf(s).AssignSamplePath(5, slow)
	must(t, err)
	for _, c := range []struct {
		url          string
		frames, rate string
	}{
		{"/audio/3", "8213", "44100"},
		{"/audio/5", "22050", "22050"},
	} {
		rec := get(c.url)
		if f, r := rec.Header().Get(FramesHeader), rec.Header().Get(SampleRateHeader); f != c.frames || r != c.rate {
			t.Errorf("%s: %s %q, %s %q; want %s and %s", c.url, FramesHeader, f, SampleRateHeader, r, c.frames, c.rate)
		}
	}

	// A source that is no longer a readable WAV is refused, as ?pcm=1 does.
	broken := filepath.Join(t.TempDir(), "broken.wav")
	must(t, os.WriteFile(broken, want, 0o644))
	_, err = hostOf(s).AssignSamplePath(6, broken)
	must(t, err)
	must(t, os.WriteFile(broken, []byte("not a WAV any more"), 0o644))
	if rec := get("/audio/6"); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unreadable source: %d, want 422", rec.Code)
	}

	rec = get("/audio/3?pcm=1")
	b := rec.Body.Bytes()
	if rec.Code != 200 || len(b) < 12 {
		t.Fatalf("pcm: %d", rec.Code)
	}
	rate, ch, frames := binary.LittleEndian.Uint32(b), binary.LittleEndian.Uint32(b[4:]), binary.LittleEndian.Uint32(b[8:])
	if rate != 44100 || ch != 1 || frames != 8213 || len(b) != 12+4*8213 {
		t.Errorf("pcm header %d Hz, %d ch, %d frames, %d bytes", rate, ch, frames, len(b))
	}
	peak := float32(0)
	for i := 12; i < len(b); i += 4 {
		v := math.Float32frombits(binary.LittleEndian.Uint32(b[i:]))
		peak = max(peak, v, -v)
	}
	if peak < 0.1 || peak > 1 {
		t.Errorf("pcm peak %v", peak)
	}

	for _, url := range []string{"/audio/4", "/audio/x", "/audio/128", "/audio/-1"} {
		if rec := get(url); rec.Code != 404 {
			t.Errorf("%s: %d, want 404", url, rec.Code)
		}
	}
}

func TestMoveCopySwap(t *testing.T) {
	s, _ := newService()
	kick := filepath.Join(kickDir, "Linn 9000 kick.wav")
	kick2 := filepath.Join(kickDir, "Linn 9000 kick001.wav")
	_, err := hostOf(s).AssignSamplePath(0, kick)
	must(t, err)
	_, err = s.SetTune(0, 3, 0)
	must(t, err)
	_, err = hostOf(s).AssignSamplePath(1, kick2)
	must(t, err)

	// Copy across banks keeps the tuning and shares the sample.
	st, err := s.CopyPad(0, 40)
	must(t, err)
	if st.Pads[40].File != "Linn 9000 kick.wav" || st.Pads[40].Coarse != 3 || st.SampleCount != 2 {
		t.Errorf("copy: %+v, %d samples", st.Pads[40], st.SampleCount)
	}
	// Swap.
	st, err = s.SwapPads(1, 40)
	must(t, err)
	if st.Pads[1].File != "Linn 9000 kick.wav" || st.Pads[40].File != "Linn 9000 kick001.wav" {
		t.Error("swap")
	}
	// Move onto a filled pad replaces it.
	st, err = s.MovePad(40, 0)
	must(t, err)
	if st.Pads[40].File != "" || st.Pads[0].File != "Linn 9000 kick001.wav" || st.Pads[0].Coarse != 0 {
		t.Errorf("move: A1 %+v", st.Pads[0])
	}
	if _, err := s.MovePad(0, 128); err == nil {
		t.Error("move to slot 128 accepted")
	}
}

func TestLoad(t *testing.T) {
	s, d := newService()

	// Cancelled: nothing changes.
	res, err := s.Load()
	must(t, err)
	if res.Path != "" || res.State.Name != DefaultName {
		t.Errorf("cancelled load: %+v", res.Path)
	}

	d.project = filepath.Join(testdata, "device", "DemoFile.xpj")
	res, err = s.Load()
	must(t, err)
	st := res.State
	if st.Name != "DemoFile" || st.Dirty || st.SampleCount != 6 || len(res.Warnings) != 0 || res.Path != d.project {
		t.Errorf("load: %q dirty %v samples %d warnings %v", st.Name, st.Dirty, st.SampleCount, res.Warnings)
	}
	// DemoFile has notes on A1-A4 only.
	for i, p := range st.Pads {
		if p.HasNotes != (i < 4) {
			t.Errorf("%s hasNotes %v", p.Label, p.HasNotes)
		}
	}
	if !strings.HasPrefix(st.Pads[0].Audio, "/audio/0?k=") || st.Pads[0].End != 44099 {
		t.Errorf("A1 %+v", st.Pads[0])
	}
	if st.SequenceCount != 2 || st.Sequences[0].Name != "Intro" || !st.Sequences[0].HasNotes {
		t.Errorf("sequences %+v", st.Sequences)
	}

	// A bad file keeps the current project.
	_, err = hostOf(s).LoadPath(filepath.Join(testdata, "device", "DemoFile_[ProjectData]", "Bass-Pure C.wav"))
	if err == nil || s.State().Name != "DemoFile" {
		t.Errorf("bad load: %v", err)
	}
}

func TestSequences(t *testing.T) {
	s, _ := newService()
	st, err := s.AddSequence(20)
	must(t, err)
	if st.SequenceCount != 2 || st.Sequences[1] != (Sequence{Slot: 20, Label: "B5", Name: "Sequence 21", BPM: 120, Bars: 2}) {
		t.Errorf("add: %+v", st.Sequences)
	}
	st, err = s.SetSequence(20, " Drop ", 140.5, 16)
	must(t, err)
	if q := st.Sequences[1]; q.Name != "Drop" || q.BPM != 140.5 || q.Bars != 16 {
		t.Errorf("set: %+v", q)
	}
	// Refused edits keep the old values and still return the state.
	for _, bad := range []struct {
		name string
		bpm  float64
		bars int
	}{{"", 120, 2}, {"x", 19, 2}, {"x", 1000, 2}, {"x", 120, 0}} {
		st, err = s.SetSequence(20, bad.name, bad.bpm, bad.bars)
		if err == nil || st.Sequences[1].Name != "Drop" {
			t.Errorf("SetSequence(%q, %v, %d): %v", bad.name, bad.bpm, bad.bars, err)
		}
	}
	if _, err := s.AddSequence(20); err == nil {
		t.Error("added twice")
	}
	st, err = s.RemoveSequence(0)
	must(t, err)
	if st.SequenceCount != 1 || st.Sequences[0].Slot != 20 {
		t.Errorf("remove: %+v", st.Sequences)
	}
	if _, err := s.RemoveSequence(20); err == nil {
		t.Error("removed the last sequence")
	}
}

// The M7 done-when through the service: MpckitT1, the M2 golden file, made
// with the calls the UI makes.
func TestGoldenT1ThroughService(t *testing.T) {
	s, _ := newService()
	calls := []func() (State, error){
		func() (State, error) { return s.SetName("MpckitT1") },
		func() (State, error) {
			return hostOf(s).AssignSamplePath(0, filepath.Join(kickDir, "Linn 9000 kick.wav"))
		},
		func() (State, error) {
			return hostOf(s).AssignSamplePath(16, filepath.Join(kickDir, "Linn 9000 kick001.wav"))
		},
		func() (State, error) { return s.SetTune(16, 5, 0) },
		func() (State, error) { return s.SetSequence(0, "Verse", 96, 8) },
		func() (State, error) { return s.AddSequence(1) },
		func() (State, error) { return s.SetSequence(1, "Chorus", 96, 4) },
	}
	for _, c := range calls {
		_, err := c()
		must(t, err)
	}
	res, err := exportTo(t, s, t.TempDir())
	must(t, err)
	if !bytes.Equal(gunzip(t, res.Path), gunzip(t, filepath.Join(testdata, "accepted", "MpckitT1.xpj"))) {
		t.Error("differs from testdata/accepted/MpckitT1.xpj")
	}
}

// copyProject copies DemoFile and its data folder into dir.
func copyProject(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(testdata, "device")
	must(t, os.Mkdir(filepath.Join(dir, "DemoFile"+project.DataSuffix), 0o755))
	b, err := os.ReadFile(filepath.Join(src, "DemoFile.xpj"))
	must(t, err)
	must(t, os.WriteFile(filepath.Join(dir, "DemoFile.xpj"), b, 0o644))
	ents, err := os.ReadDir(filepath.Join(src, "DemoFile"+project.DataSuffix))
	must(t, err)
	for _, e := range ents {
		b, err := os.ReadFile(filepath.Join(src, "DemoFile"+project.DataSuffix, e.Name()))
		must(t, err)
		must(t, os.WriteFile(filepath.Join(dir, "DemoFile"+project.DataSuffix, e.Name()), b, 0o644))
	}
	return filepath.Join(dir, "DemoFile.xpj")
}

func TestMissingFiles(t *testing.T) {
	dir := t.TempDir()
	path := copyProject(t, dir)
	must(t, os.Remove(filepath.Join(dir, "DemoFile"+project.DataSuffix, "80s-Snr-Dreamer SA.wav")))

	s, _ := newService()
	res, err := hostOf(s).LoadPath(path)
	must(t, err)
	st := res.State
	if st.MissingCount != 1 {
		t.Fatalf("missing count %d, want 1", st.MissingCount)
	}
	a2 := st.Pads[1]
	if !a2.Missing || a2.Audio != "" || !strings.HasSuffix(a2.Source, "80s-Snr-Dreamer SA.wav") {
		t.Errorf("A2 %+v", a2)
	}
	if st.Pads[0].Missing || st.Pads[0].Audio == "" {
		t.Error("A1 marked missing")
	}

	// Export names the missing file and writes nothing.
	out := t.TempDir()
	if _, err := exportTo(t, s, out); err == nil || !strings.Contains(err.Error(), "80s-Snr-Dreamer SA.wav") {
		t.Errorf("export with a missing file: %v", err)
	}
	if ents, _ := os.ReadDir(out); len(ents) != 0 {
		t.Error("export wrote files")
	}

	// Replacing the pad's sample fixes it.
	st, err = hostOf(s).AssignSamplePath(1, filepath.Join(kickDir, "Linn 9000 kick.wav"))
	must(t, err)
	if st.MissingCount != 0 || st.Pads[1].Missing {
		t.Error("still missing after replace")
	}
	if _, err := exportTo(t, s, out); err != nil {
		t.Error(err)
	}
}

func TestCloseWindow(t *testing.T) {
	s, _ := newService()
	s.CloseWindow() // no closer set: does nothing
	closed := false
	hostOf(s).SetCloser(func() { closed = true })
	if hostOf(s).Dirty() {
		t.Error("new project dirty")
	}
	_, _ = s.SetName("Other")
	if !hostOf(s).Dirty() {
		t.Error("not dirty after rename")
	}
	s.CloseWindow()
	if !closed {
		t.Error("closer not called")
	}
}

func TestLinkedFileIsMissing(t *testing.T) {
	// A link in the data folder is shown as missing, as export treats it.
	dir := t.TempDir()
	path := copyProject(t, dir)
	snare := filepath.Join(dir, "DemoFile"+project.DataSuffix, "80s-Snr-Dreamer SA.wav")
	outside := filepath.Join(dir, "outside.wav")
	must(t, os.Rename(snare, outside))
	if err := os.Symlink(outside, snare); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
	s, _ := newService()
	res, err := hostOf(s).LoadPath(path)
	must(t, err)
	if st := res.State; st.MissingCount != 1 || !st.Pads[1].Missing {
		t.Errorf("missing count %d, A2 missing %v; want 1, true", st.MissingCount, st.Pads[1].Missing)
	}
}

// TestBoundMethods pins the methods the window can call. Wails binds every
// exported method of Service by reflection, so each one is reachable from
// any script in the window by name.
func TestBoundMethods(t *testing.T) {
	want := []string{
		"About", "AddSequence", "AssignSampleDialog", "CancelExport",
		"ChooseExportFolder", "ChooseZipPath", "ClearPad", "CloseWindow",
		"CopyPad", "ExportFolder", "ExportZip", "Load", "MovePad", "New",
		"RemoveSequence", "SetMuteGroup", "SetName", "SetSequence", "SetTune",
		"State", "SwapPads",
	}
	typ := reflect.TypeOf(&Service{})
	var got []string
	for i := range typ.NumMethod() {
		got = append(got, typ.Method(i).Name)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Service's exported methods are\n  %v\nwant\n  %v\n"+
			"Every exported method of Service can be called by any script in the window. "+
			"A new one must not take a file path (Go-only methods belong on Host), "+
			"and must be added to this list on purpose.", got, want)
	}
}

func TestCancelExport(t *testing.T) {
	// Declining "Replace existing files?" forgets the target, so a later
	// overwrite cannot reach it without the dialog.
	s, d := newService()
	d.folder, d.zip = t.TempDir(), filepath.Join(t.TempDir(), "kit.zip")
	_, err := s.ChooseExportFolder()
	must(t, err)
	_, err = s.ChooseZipPath()
	must(t, err)
	s.CancelExport()
	for _, export := range []func(bool) (ExportResult, error){s.ExportFolder, s.ExportZip} {
		if _, err := export(true); err == nil || err.Error() != "choose where to export first" {
			t.Errorf("overwrite after cancel: %v", err)
		}
	}
	if ents, _ := os.ReadDir(d.folder); len(ents) != 0 {
		t.Errorf("export after cancel wrote %d entries", len(ents))
	}
}
