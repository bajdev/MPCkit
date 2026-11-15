package xpj

import (
	"bytes"
	"compress/gzip"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// firstDiff describes where two byte slices first differ.
func firstDiff(a, b []byte) string {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			lo := max(0, i-40)
			return "at byte " + itoa(i) + ":\n got  " + quote(a[lo:min(len(a), i+40)]) + "\n want " + quote(b[lo:min(len(b), i+40)])
		}
	}
	return "lengths " + itoa(len(a)) + " and " + itoa(len(b))
}

func itoa(i int) string { return NewInt(int64(i)).Text }
func quote(b []byte) string {
	return strings.ReplaceAll(string(b), "\n", `\n`)
}

// The M1 milestone: every device-saved project re-writes byte-identical.
func TestRoundTripDeviceProjects(t *testing.T) {
	for _, name := range []string{"DemoBlank", "Demo02", "DemoFile"} {
		t.Run(name, func(t *testing.T) {
			roundTrip(t, filepath.Join(testdata, "device", name+".xpj"))
		})
	}
}

// Files from the reference writer that the device accepted round-trip too.
func TestRoundTripAcceptedProjects(t *testing.T) {
	paths, _ := filepath.Glob(filepath.Join(testdata, "accepted", "*.xpj"))
	more, _ := filepath.Glob(filepath.Join(testdata, "wav-formats", "*.xpj"))
	paths = append(paths, more...)
	if len(paths) < 5 {
		t.Fatalf("found only %d accepted projects", len(paths))
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) { roundTrip(t, p) })
	}
}

func roundTrip(t *testing.T, path string) {
	t.Helper()
	want := decompress(t, path)
	f, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("re-encoded file differs %s", firstDiff(got, want))
	}

	// Through gzip and back, as an export would.
	var z bytes.Buffer
	if err := f.Write(&z); err != nil {
		t.Fatal(err)
	}
	f2, err := Read(&z)
	if err != nil {
		t.Fatal(err)
	}
	got2, _ := f2.Encode()
	if !bytes.Equal(got2, want) {
		t.Fatalf("gzip round trip differs %s", firstDiff(got2, want))
	}
}

func TestDeviceProjectShape(t *testing.T) {
	f, err := ReadFile(filepath.Join(testdata, "device", "DemoBlank.xpj"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.Header, "|") != strings.Join(TestedHeader, "|") {
		t.Errorf("header = %q", f.Header)
	}
	d, err := f.Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Members) != 66 {
		t.Errorf("data has %d keys, want 66", len(d.Members))
	}
	if v, _ := d.Lookup("version").Int(); v != 28 {
		t.Errorf("data.version = %d, want 28", v)
	}
	pads, err := d.Get("tracks[0].program.drum.instruments")
	if err != nil {
		t.Fatal(err)
	}
	if len(pads.Items) != 128 {
		t.Errorf("%d pads, want 128", len(pads.Items))
	}
	seq, err := d.Get("sequences[0].value")
	if err != nil {
		t.Fatal(err)
	}
	if seq.Lookup("name").Text != "Sequence 01" {
		t.Errorf("sequence name = %q", seq.Lookup("name").Text)
	}
	if bpm, _ := seq.Lookup("bpm").Float(); bpm != 128 {
		t.Errorf("bpm = %v", bpm)
	}
}

func TestGzipHeader(t *testing.T) {
	f := &File{Header: TestedHeader, Root: NewObject()}
	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		t.Fatal(err)
	}
	// ID, CM=deflate, FLG=0, MTIME=0, XFL=0, OS=3: the same ten bytes the device writes.
	want := []byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 3}
	if got := b.Bytes()[:10]; !bytes.Equal(got, want) {
		t.Errorf("gzip header = % x, want % x", got, want)
	}
}

func TestLayout(t *testing.T) {
	in := `{"a": [1, 2.50, {}], "b": {"x": [], "y": "s<&>/"}, "c": null, "d": true, "e": -1e-05}`
	n, err := ParseJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	got, err := n.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n\"a\": [\n1,\n2.50,\n{}\n],\n\"b\": {\n\"x\": [],\n\"y\": \"s<&>/\"\n},\n\"c\": null,\n\"d\": true,\n\"e\": -1e-05\n}"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestKeyOrderAndDuplicates(t *testing.T) {
	n, err := ParseJSON([]byte(`{"z": 1, "a": 2, "z": 3}`))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := n.MarshalJSON()
	if want := "{\n\"z\": 1,\n\"a\": 2,\n\"z\": 3\n}"; string(got) != want {
		t.Errorf("got %q", got)
	}
}

func TestStringEscapes(t *testing.T) {
	s := "q\" b\\ n\n t\t c\x01 é ✓"
	n := NewString(s)
	got, _ := n.MarshalJSON()
	if want := `"q\" b\\ n\n t\t c\u0001 é ✓"`; string(got) != want {
		t.Errorf("got %s want %s", got, want)
	}
	back, err := ParseJSON(got)
	if err != nil || back.Text != s {
		t.Errorf("read back %q, %v", back.Text, err)
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{``, `{`, `{"a": }`, `[1,]`, `{} {}`, `{"a": 1} x`, `nul`} {
		if _, err := ParseJSON([]byte(in)); err == nil {
			t.Errorf("ParseJSON(%q) succeeded", in)
		}
	}
	if _, err := Decode([]byte("ACVS\n1\n2\n3\n")); err == nil {
		t.Error("short header accepted")
	}
	if _, err := Decode([]byte("XXXX\n1\n2\n3\n4\n{}")); err == nil {
		t.Error("bad magic accepted")
	}
	if _, err := Decode([]byte("ACVS\n1\n2\n3\n4\n[]")); err == nil {
		t.Error("array body accepted")
	}
	if _, err := Read(strings.NewReader("not gzip")); err == nil {
		t.Error("non-gzip accepted")
	}
}

func TestInvalidNumberText(t *testing.T) {
	for _, s := range []string{"", "1.", ".5", "01", "+1", "1e", "true", `"1"`, "NaN", "1 "} {
		n := &Node{Kind: Number, Text: s}
		if _, err := n.MarshalJSON(); err == nil {
			t.Errorf("number text %q accepted", s)
		}
	}
}

func TestFormatFloat(t *testing.T) {
	// Expected values are Python's repr, which the reference writer used.
	cases := map[float64]string{
		96:                     "96.0",
		20:                     "20.0",
		0:                      "0.0",
		math.Copysign(0, -1):   "-0.0",
		12.399999618530273:     "12.399999618530273",
		1e-05:                  "1e-05",
		0.0001:                 "0.0001",
		1e16:                   "1e+16",
		1.5e16:                 "1.5e+16",
		123456789012345.6:      "123456789012345.6",
		1e15:                   "1000000000000000.0",
		-2.5:                   "-2.5",
		0.7079457640647888:     "0.7079457640647888",
		5e-324:                 "5e-324",
		1.7976931348623157e308: "1.7976931348623157e+308",
		0.1:                    "0.1",
		100:                    "100.0",
		9999999999999998:       "9999999999999998.0",
	}
	for f, want := range cases {
		if got := FormatFloat(f); got != want {
			t.Errorf("FormatFloat(%v) = %q, want %q", f, got, want)
		}
	}
	// Tuning +12 semitones 40 cents, as the device writes it.
	if got := NewFloat32(12 + 40.0/100).Text; got != "12.399999618530273" {
		t.Errorf("NewFloat32(12.4) = %s", got)
	}
	if got := NewInt(-1).Text; got != "-1" {
		t.Errorf("NewInt(-1) = %s", got)
	}
}

func TestPaths(t *testing.T) {
	n, err := ParseJSON([]byte(`{"a": [{"b": [10, 20]}, {"c": "x"}], "d": {"e": 1.0}}`))
	if err != nil {
		t.Fatal(err)
	}
	get := func(p string) *Node {
		t.Helper()
		v, err := n.Get(p)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if get("a[0].b[1]").Text != "20" || get("a[1].c").Text != "x" || get("d.e").Text != "1.0" {
		t.Error("Get returned wrong nodes")
	}
	if get("") != n {
		t.Error("empty path is not the root")
	}
	for _, p := range []string{"a[2]", "a[0].x", "d[0]", "d.e.f", "a[", "a[-1]", "a[x]", ".a", "a..b", "a.", "a]", "a.[0]"} {
		if _, err := n.Get(p); err == nil {
			t.Errorf("Get(%q) succeeded", p)
		}
	}

	if err := n.Set("a[0].b[0]", NewInt(4)); err != nil {
		t.Fatal(err)
	}
	if err := n.Set("d.e", NewFloat(96)); err != nil {
		t.Fatal(err)
	}
	if err := n.Set("d.new", NewString("n")); err != nil {
		t.Fatal(err)
	}
	if err := n.Set("a[5]", NewNull()); err == nil {
		t.Error("Set past array end succeeded")
	}
	if err := n.Set("", NewNull()); err == nil {
		t.Error("Set root succeeded")
	}
	got, _ := n.MarshalJSON()
	want := "{\n\"a\": [\n{\n\"b\": [\n4,\n20\n]\n},\n{\n\"c\": \"x\"\n}\n],\n\"d\": {\n\"e\": 96.0,\n\"new\": \"n\"\n}\n}"
	if string(got) != want {
		t.Errorf("got\n%s", got)
	}
}

func TestArrayEditsAndClone(t *testing.T) {
	n, _ := ParseJSON([]byte(`{"s": [{"k": 0}]}`))
	arr, _ := n.Get("s")
	c := arr.Items[0].Clone()
	if err := c.SetKey("k", NewInt(1)); err != nil {
		t.Fatal(err)
	}
	if arr.Items[0].Lookup("k").Text != "0" {
		t.Fatal("clone shares data with its source")
	}
	if err := arr.Insert(0, c); err != nil {
		t.Fatal(err)
	}
	if err := arr.Append(NewInt(9)); err != nil {
		t.Fatal(err)
	}
	if err := arr.Insert(5, NewNull()); err == nil {
		t.Error("insert out of range succeeded")
	}
	if err := arr.Remove(1); err != nil {
		t.Fatal(err)
	}
	if err := arr.Remove(2); err == nil {
		t.Error("remove out of range succeeded")
	}
	got, _ := n.MarshalJSON()
	if want := "{\n\"s\": [\n{\n\"k\": 1\n},\n9\n]\n}"; string(got) != want {
		t.Errorf("got\n%s", got)
	}

	obj, _ := n.Get("s[0]")
	if !obj.DeleteKey("k") || obj.DeleteKey("k") {
		t.Error("DeleteKey")
	}
	if got, _ := obj.MarshalJSON(); string(got) != "{}" {
		t.Errorf("emptied object = %s", got)
	}

	f := &File{Header: TestedHeader, Root: n}
	fc := f.Clone()
	fc.Header[0] = "X"
	_ = fc.Root.SetKey("s", NewNull())
	if f.Header[0] != "ACVS" || n.Lookup("s").Kind != Array {
		t.Error("File.Clone shares data with its source")
	}
}

func TestNumberAccessors(t *testing.T) {
	for text, want := range map[string]int64{"4": 4, "-1": -1, "8212": 8212, "20.0": 20} {
		if got, err := (&Node{Kind: Number, Text: text}).Int(); err != nil || got != want {
			t.Errorf("Int(%s) = %d, %v", text, got, err)
		}
	}
	if _, err := (&Node{Kind: Number, Text: "1.5"}).Int(); err == nil {
		t.Error("Int(1.5) succeeded")
	}
	if _, err := NewString("1").Int(); err == nil {
		t.Error("Int on a string succeeded")
	}
	if f, err := NewFloat32(0.1).Float(); err != nil || f != float64(float32(0.1)) {
		t.Errorf("Float = %v, %v", f, err)
	}
}

func TestNilNode(t *testing.T) {
	var n *Node
	if _, err := n.Int(); err == nil {
		t.Error("Int on nil: no error")
	}
	if _, err := n.Float(); err == nil {
		t.Error("Float on nil: no error")
	}
	if n.Lookup("a") != nil {
		t.Error("Lookup on nil: not nil")
	}
	if _, err := n.Get("a[0]"); err == nil {
		t.Error("Get on nil: no error")
	}
}

// gzipPadded returns a gzip stream that decompresses to a minimal project
// whose JSON body is padded with spaces to exactly size bytes.
func gzipPadded(t *testing.T, size int) []byte {
	t.Helper()
	head := "ACVS\n1.3.0.12\nSerialisableProjectData\njson\nLinux\n{\"data\": {}"
	if size < len(head)+1 {
		t.Fatalf("size %d too small", size)
	}
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	if _, err := io.WriteString(zw, head); err != nil {
		t.Fatal(err)
	}
	spaces := bytes.Repeat([]byte(" "), 1<<20)
	for left := size - len(head) - 1; left > 0; {
		n := min(left, len(spaces))
		if _, err := zw.Write(spaces[:n]); err != nil {
			t.Fatal(err)
		}
		left -= n
	}
	if _, err := io.WriteString(zw, "}"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestDecompressionLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("decompresses 512 MB")
	}
	// One byte over: refused.
	_, err := Read(bytes.NewReader(gzipPadded(t, MaxDecompressed+1)))
	want := "xpj: project is larger than 256 MB when decompressed"
	if err == nil || err.Error() != want {
		t.Fatalf("over the limit: got %v, want %q", err, want)
	}
	// Exactly at the limit: still decodes.
	f, err := Read(bytes.NewReader(gzipPadded(t, MaxDecompressed)))
	if err != nil {
		t.Fatalf("at the limit: %v", err)
	}
	if _, err := f.Data(); err != nil {
		t.Fatalf("at the limit: %v", err)
	}
}
