package wav

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testdata = "../../testdata"

// The WAV test pack: every file is one second long at its own rate.
func TestWavFormatsPack(t *testing.T) {
	cases := map[string]struct{ format, ch, rate, bits int }{
		"MpckitW1/22k 16bit mono.wav":         {FormatPCM, 1, 22050, 16},
		"MpckitW1/44k 16bit mono.wav":         {FormatPCM, 1, 44100, 16},
		"MpckitW1/44k 16bit stereo.wav":       {FormatPCM, 2, 44100, 16},
		"MpckitW1/44k 24bit stereo.wav":       {FormatPCM, 2, 44100, 24},
		"MpckitW1/48k 16bit stereo.wav":       {FormatPCM, 2, 48000, 16},
		"MpckitW1/48k 24bit stereo.wav":       {FormatPCM, 2, 48000, 24},
		"MpckitW1/96k 24bit stereo.wav":       {FormatPCM, 2, 96000, 24},
		"MpckitW2/44k 8bit mono.wav":          {FormatPCM, 1, 44100, 8},
		"MpckitW2/44k 32bit int stereo.wav":   {FormatPCM, 2, 44100, 32},
		"MpckitW2/44k 32bit float stereo.wav": {FormatFloat, 2, 44100, 32},
	}
	for name, want := range cases {
		proj, file, _ := strings.Cut(name, "/")
		info, err := ReadFile(filepath.Join(testdata, "wav-formats", proj+"_[ProjectData]", file))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if info.Format != want.format || info.Channels != want.ch || info.SampleRate != want.rate || info.Bits != want.bits {
			t.Errorf("%s: %+v", name, info)
		}
		if info.Frames != int64(want.rate) || info.Duration() != time.Second {
			t.Errorf("%s: %d frames, %v", name, info.Frames, info.Duration())
		}
		if err := info.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Device-saved files carry extra chunks (atem, smpl, meta) around the data.
func TestDeviceSavedFiles(t *testing.T) {
	cases := map[string]int64{
		"device/Demo02_[ProjectData]/Linn 9000 kick.wav":          8213,
		"device/DemoFile_[ProjectData]/80s-Kik-Dreamer2 SA.wav":   44100,
		"device/DemoFile_[ProjectData]/80s-Snr-Dreamer SA.wav":    27000,
		"device/DemoFile_[ProjectData]/80s-Hat-Dreamer1 SA.wav":   22446,
		"device/DemoFile_[ProjectData]/80s-Hat-Dreamer2 SA.wav":   108000,
		"device/DemoFile_[ProjectData]/PadLp-London Cm114 SA.wav": 371368,
		"device/DemoFile_[ProjectData]/Bass-Pure C.wav":           132557,
		"accepted/MpckitT2_[ProjectData]/Linn 9000 kick.wav":      8213,
	}
	for name, frames := range cases {
		info, err := ReadFile(filepath.Join(testdata, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if info.Frames != frames {
			t.Errorf("%s: %d frames, want %d (sliceInfo.End + 1)", name, info.Frames, frames)
		}
		if err := info.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

type chunk struct {
	id   string
	body []byte
}

func riff(form string, chunks ...chunk) []byte {
	var body bytes.Buffer
	body.WriteString(form)
	for _, c := range chunks {
		body.WriteString(c.id)
		binary.Write(&body, binary.LittleEndian, uint32(len(c.body)))
		body.Write(c.body)
		if len(c.body)%2 == 1 {
			body.WriteByte(0)
		}
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(body.Len()))
	b.Write(body.Bytes())
	return b.Bytes()
}

func fmtChunk(format, ch, rate, bits int) []byte {
	var b bytes.Buffer
	align := ch * ((bits + 7) / 8)
	for _, v := range []any{uint16(format), uint16(ch), uint32(rate), uint32(rate * align), uint16(align), uint16(bits)} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	return b.Bytes()
}

func extensible(sub, ch, rate, bits int) []byte {
	b := fmtChunk(FormatExtensible, ch, rate, bits)
	ext := make([]byte, 24)
	binary.LittleEndian.PutUint16(ext[0:2], 22)
	binary.LittleEndian.PutUint16(ext[2:4], uint16(bits))
	binary.LittleEndian.PutUint16(ext[8:10], uint16(sub))
	copy(ext[10:], "\x00\x00\x00\x00\x10\x00\x80\x00\x00\xaa\x00\x38\x9b\x71")
	return append(b, ext...)
}

func read(b []byte) (Info, error) { return Read(bytes.NewReader(b), int64(len(b))) }

func TestSyntheticHeaders(t *testing.T) {
	data := make([]byte, 600)
	ok := []struct {
		name   string
		file   []byte
		format int
		frames int64
	}{
		{"extensible PCM 24-bit", riff("WAVE", chunk{"fmt ", extensible(FormatPCM, 2, 48000, 24)}, chunk{"data", data}), FormatPCM, 100},
		{"extensible float", riff("WAVE", chunk{"fmt ", extensible(FormatFloat, 1, 44100, 32)}, chunk{"data", data}), FormatFloat, 150},
		{"odd chunk before fmt", riff("WAVE", chunk{"LIST", []byte("abc")}, chunk{"fmt ", fmtChunk(1, 1, 44100, 16)}, chunk{"data", data}), FormatPCM, 300},
		{"8-bit mono", riff("WAVE", chunk{"fmt ", fmtChunk(1, 1, 22050, 8)}, chunk{"data", data[:599]}), FormatPCM, 599},
	}
	for _, c := range ok {
		info, err := read(c.file)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if info.Format != c.format || info.Frames != c.frames {
			t.Errorf("%s: %+v", c.name, info)
		}
		if err := info.Validate(); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}

	// Data size left at zero by a streaming writer: count what is there.
	f := riff("WAVE", chunk{"fmt ", fmtChunk(1, 1, 44100, 16)}, chunk{"data", data})
	binary.LittleEndian.PutUint32(f[len(f)-len(data)-4:], 0)
	if info, err := read(f); err != nil || info.Frames != 300 {
		t.Errorf("zero data size: %+v %v", info, err)
	}
	// Truncated data: count what is there.
	f = riff("WAVE", chunk{"fmt ", fmtChunk(1, 1, 44100, 16)}, chunk{"data", data})
	if info, err := read(f[:len(f)-100]); err != nil || info.Frames != 250 {
		t.Errorf("truncated: %+v %v", info, err)
	}

	unreadable := map[string][]byte{
		"empty":     nil,
		"not riff":  []byte("OggS0000WAVE"),
		"RF64":      append([]byte("RF64\xff\xff\xff\xffWAVE"), make([]byte, 20)...),
		"no fmt":    riff("WAVE", chunk{"data", data}),
		"no data":   riff("WAVE", chunk{"fmt ", fmtChunk(1, 1, 44100, 16)}),
		"short fmt": riff("WAVE", chunk{"fmt ", make([]byte, 10)}, chunk{"data", data}),
		"zero rate": riff("WAVE", chunk{"fmt ", fmtChunk(1, 1, 0, 16)}, chunk{"data", data}),
		"bad align": riff("WAVE", chunk{"fmt ", append(fmtChunk(1, 2, 44100, 16)[:12], 3, 0, 16, 0)}, chunk{"data", data}),
		"short ext": riff("WAVE", chunk{"fmt ", fmtChunk(FormatExtensible, 1, 44100, 16)}, chunk{"data", data}),
	}
	for name, f := range unreadable {
		if _, err := read(f); err == nil {
			t.Errorf("%s: read succeeded", name)
		}
	}

	refused := map[string][]byte{
		"ADPCM":        riff("WAVE", chunk{"fmt ", fmtChunk(2, 1, 44100, 4)}, chunk{"data", data}),
		"3 channels":   riff("WAVE", chunk{"fmt ", fmtChunk(1, 3, 44100, 16)}, chunk{"data", data}),
		"64-bit float": riff("WAVE", chunk{"fmt ", fmtChunk(3, 1, 44100, 64)}, chunk{"data", data}),
		"12-bit":       riff("WAVE", chunk{"fmt ", fmtChunk(1, 1, 44100, 12)}, chunk{"data", data}),
		"no audio":     riff("WAVE", chunk{"fmt ", fmtChunk(1, 1, 44100, 16)}, chunk{"data", nil}),
	}
	for name, f := range refused {
		info, err := read(f)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if info.Validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	long := Info{Format: FormatPCM, Channels: 1, SampleRate: 44100, Bits: 16, BlockAlign: 2, Frames: 44100*20*60 + 1}
	if long.Validate() == nil {
		t.Error("over 20 minutes accepted")
	}
	long.Frames--
	if err := long.Validate(); err != nil {
		t.Errorf("exactly 20 minutes: %v", err)
	}
}

// Every file in the WAV test pack holds the same tone; decoded, the 44.1 kHz
// files must agree with the 16-bit control to within 8-bit precision.
func TestDecodeAgreesAcrossFormats(t *testing.T) {
	dir := filepath.Join(testdata, "wav-formats")
	_, ref, err := DecodeFile(filepath.Join(dir, "MpckitW1_[ProjectData]", "44k 16bit mono.wav"))
	if err != nil {
		t.Fatal(err)
	}
	peak := float32(0)
	for _, v := range ref[0] {
		peak = max(peak, v)
	}
	if peak < 0.45 || peak > 0.55 {
		t.Errorf("control peak %v, want about 0.5", peak)
	}
	for _, name := range []string{
		"MpckitW1_[ProjectData]/44k 16bit stereo.wav",
		"MpckitW1_[ProjectData]/44k 24bit stereo.wav",
		"MpckitW2_[ProjectData]/44k 8bit mono.wav",
		"MpckitW2_[ProjectData]/44k 32bit int stereo.wav",
		"MpckitW2_[ProjectData]/44k 32bit float stereo.wav",
	} {
		info, ch, err := DecodeFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(ch) != info.Channels || int64(len(ch[0])) != info.Frames {
			t.Errorf("%s: %d channels of %d frames", name, len(ch), len(ch[0]))
			continue
		}
		worst := float32(0)
		for c := range ch {
			for i, v := range ch[c] {
				d := v - ref[0][i]
				worst = max(worst, d, -d)
			}
		}
		if worst > 1.0/64 {
			t.Errorf("%s: differs from the control by up to %v", name, worst)
		}
	}
}

func TestDecodeExtremes(t *testing.T) {
	cases := []struct {
		bits   int
		format int
		raw    []byte
		want   []float32
	}{
		{8, FormatPCM, []byte{0, 128, 255}, []float32{-1, 0, 127.0 / 128}},
		{16, FormatPCM, []byte{0, 0x80, 0, 0, 0xff, 0x7f}, []float32{-1, 0, 32767.0 / 32768}},
		{24, FormatPCM, []byte{0, 0, 0x80, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}, []float32{-1, -1.0 / (1 << 23), 8388607.0 / (1 << 23)}},
		{32, FormatPCM, []byte{0, 0, 0, 0x80, 0, 0, 0, 0}, []float32{-1, 0}},
		{32, FormatFloat, []byte{0, 0, 0x80, 0x3f, 0, 0, 0, 0xbf}, []float32{1, -0.5}},
	}
	for _, c := range cases {
		info := Info{Format: c.format, Channels: 1, Bits: c.bits, BlockAlign: c.bits / 8}
		got := Decode(info, c.raw)[0]
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("%d-bit format %d sample %d = %v, want %v", c.bits, c.format, i, got[i], c.want[i])
			}
		}
	}
}
