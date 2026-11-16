package wav

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// DecodeFile reads the whole WAV at path and returns its samples as 32-bit
// floats in [-1, 1], one slice per channel. It is the preview fallback for
// formats a webview cannot decode (build plan section 9).
func DecodeFile(path string) (Info, [][]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Info{}, nil, err
	}
	info, err := Read(f, st.Size())
	if err != nil {
		return Info{}, nil, err
	}
	if err := info.Validate(); err != nil {
		return Info{}, nil, err
	}
	raw := make([]byte, info.Frames*int64(info.BlockAlign))
	if _, err := f.ReadAt(raw, info.DataOffset); err != nil && err != io.EOF {
		return Info{}, nil, fmt.Errorf("reading samples: %w", err)
	}
	return info, Decode(info, raw), nil
}

// Decode converts raw sample data described by info into per-channel floats.
// info must have passed Validate.
func Decode(info Info, raw []byte) [][]float32 {
	le := binary.LittleEndian
	width := info.BlockAlign / info.Channels
	frames := len(raw) / info.BlockAlign
	out := make([][]float32, info.Channels)
	for c := range out {
		out[c] = make([]float32, frames)
	}
	var sample func(b []byte) float32
	switch {
	case info.Format == FormatFloat:
		sample = func(b []byte) float32 { return math.Float32frombits(le.Uint32(b)) }
	case info.Bits == 8: // unsigned, centred on 128
		sample = func(b []byte) float32 { return (float32(b[0]) - 128) / 128 }
	case info.Bits == 16:
		sample = func(b []byte) float32 { return float32(int16(le.Uint16(b))) / (1 << 15) }
	case info.Bits == 24:
		sample = func(b []byte) float32 {
			v := int32(uint32(b[0])<<8|uint32(b[1])<<16|uint32(b[2])<<24) >> 8
			return float32(v) / (1 << 23)
		}
	case info.Bits == 32:
		sample = func(b []byte) float32 { return float32(float64(int32(le.Uint32(b))) / (1 << 31)) }
	}
	for i := 0; i < frames; i++ {
		frame := raw[i*info.BlockAlign:]
		for c := 0; c < info.Channels; c++ {
			out[c][i] = sample(frame[c*width:])
		}
	}
	return out
}
