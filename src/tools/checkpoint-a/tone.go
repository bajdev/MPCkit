package main

import (
	"bytes"
	"encoding/binary"
	"math"
)

// tone returns the WAV test pack's tone (0.8 s at 440 Hz, then 0.2 s at
// 880 Hz as an end marker) as a WAV file of integer PCM, optionally with the
// WAVE_FORMAT_EXTENSIBLE header many audio tools write. bits is 16 or 24.
func tone(rate, bits, channels int, ext bool) []byte {
	le := binary.LittleEndian
	width := bits / 8
	var data bytes.Buffer
	fade := 0.005 * float64(rate)
	for i := 0; i < rate; i++ {
		t := float64(i) / float64(rate)
		f := 440.0
		if t >= 0.8 {
			f = 880
		}
		env := math.Min(1, math.Min(float64(i)/fade, float64(rate-1-i)/fade))
		v := 0.5 * env * math.Sin(2*math.Pi*f*t)
		s := int32(math.Round(v * float64(int32(1)<<(bits-1)-1)))
		var b [4]byte
		le.PutUint32(b[:], uint32(s))
		for c := 0; c < channels; c++ {
			data.Write(b[:width])
		}
	}

	align := channels * width
	var fmtc bytes.Buffer
	tag := uint16(1)
	if ext {
		tag = 0xFFFE
	}
	for _, v := range []any{tag, uint16(channels), uint32(rate), uint32(rate * align), uint16(align), uint16(bits)} {
		binary.Write(&fmtc, le, v)
	}
	if ext {
		mask := uint32(4) // front centre
		if channels == 2 {
			mask = 3 // front left, front right
		}
		for _, v := range []any{uint16(22), uint16(bits), mask} {
			binary.Write(&fmtc, le, v)
		}
		// KSDATAFORMAT_SUBTYPE_PCM 00000001-0000-0010-8000-00aa00389b71
		fmtc.Write([]byte{1, 0, 0, 0, 0, 0, 0x10, 0, 0x80, 0, 0, 0xaa, 0, 0x38, 0x9b, 0x71})
	}

	var body bytes.Buffer
	body.WriteString("WAVEfmt ")
	binary.Write(&body, le, uint32(fmtc.Len()))
	body.Write(fmtc.Bytes())
	body.WriteString("data")
	binary.Write(&body, le, uint32(data.Len()))
	body.Write(data.Bytes())
	if data.Len()%2 == 1 {
		body.WriteByte(0)
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	binary.Write(&out, le, uint32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}
