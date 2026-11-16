// Package wav reads WAV files. ReadFile reads only the header (format,
// channels, sample rate, bit depth and frame count), and Validate says
// whether MPCkit and the device can use the file. DecodeFile also decodes
// the samples, for previews the webview cannot play itself.
package wav

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Format codes from the fmt chunk.
const (
	FormatPCM        = 1
	FormatFloat      = 3
	FormatExtensible = 0xFFFE
)

// Limits for a sample MPCkit will assign (build plan 4.3 and section 7).
const (
	MaxChannels = 2
	MaxDuration = 20 * time.Minute
)

// Info describes a WAV file.
type Info struct {
	Format     int // FormatPCM or FormatFloat (the sub-format of an extensible header)
	Extensible bool
	Channels   int
	SampleRate int
	Bits       int
	BlockAlign int
	Frames     int64
	DataOffset int64 // byte offset of the first sample
	DataSize   int64 // bytes of sample data actually present
}

// Duration returns the length of the audio.
func (i Info) Duration() time.Duration {
	if i.SampleRate == 0 {
		return 0
	}
	return time.Duration(i.Frames) * time.Second / time.Duration(i.SampleRate)
}

// ReadFile reads the header of the WAV file at path.
func ReadFile(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Info{}, err
	}
	return Read(f, st.Size())
}

// Read reads a WAV header from r, which holds size bytes.
func Read(r io.ReaderAt, size int64) (Info, error) {
	var hdr [12]byte
	if _, err := r.ReadAt(hdr[:], 0); err != nil {
		return Info{}, errors.New("not a WAV file: too short")
	}
	if string(hdr[0:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		if string(hdr[0:4]) == "RF64" {
			return Info{}, errors.New("RF64 WAV files are not supported")
		}
		return Info{}, errors.New("not a WAV file")
	}

	var info Info
	var haveFmt, haveData bool
	pos := int64(12)
	for pos+8 <= size && !(haveFmt && haveData) {
		var ch [8]byte
		if _, err := r.ReadAt(ch[:], pos); err != nil {
			return Info{}, fmt.Errorf("reading chunk header: %w", err)
		}
		id := string(ch[0:4])
		n := int64(binary.LittleEndian.Uint32(ch[4:8]))
		body := pos + 8
		switch id {
		case "fmt ":
			if n < 16 || body+n > size {
				return Info{}, errors.New("fmt chunk is too short")
			}
			buf := make([]byte, n)
			if _, err := r.ReadAt(buf, body); err != nil {
				return Info{}, fmt.Errorf("reading fmt chunk: %w", err)
			}
			if err := info.parseFmt(buf); err != nil {
				return Info{}, err
			}
			haveFmt = true
		case "data":
			info.DataOffset = body
			// Some writers leave the size at 0 or 0xFFFFFFFF; count what is there.
			info.DataSize = min(n, size-body)
			if n == 0 || n == 0xFFFFFFFF {
				info.DataSize = size - body
			}
			haveData = true
		}
		pos = body + n + n&1
	}
	if !haveFmt {
		return Info{}, errors.New("WAV file has no fmt chunk")
	}
	if !haveData {
		return Info{}, errors.New("WAV file has no data chunk")
	}
	info.Frames = info.DataSize / int64(info.BlockAlign)
	return info, nil
}

func (i *Info) parseFmt(b []byte) error {
	le := binary.LittleEndian
	i.Format = int(le.Uint16(b[0:2]))
	i.Channels = int(le.Uint16(b[2:4]))
	i.SampleRate = int(le.Uint32(b[4:8]))
	i.BlockAlign = int(le.Uint16(b[12:14]))
	i.Bits = int(le.Uint16(b[14:16]))
	if i.Format == FormatExtensible {
		// cbSize(2) validBits(2) channelMask(4) subFormat GUID(16): the
		// first two bytes of the GUID are the real format code.
		if len(b) < 40 {
			return errors.New("extensible fmt chunk is too short")
		}
		i.Extensible = true
		i.Format = int(le.Uint16(b[24:26]))
	}
	if i.Channels == 0 || i.SampleRate == 0 || i.BlockAlign == 0 {
		return errors.New("WAV header has zero channels, rate or block size")
	}
	if i.BlockAlign != i.Channels*((i.Bits+7)/8) {
		return fmt.Errorf("WAV block size %d does not match %d channels of %d bits", i.BlockAlign, i.Channels, i.Bits)
	}
	return nil
}

// Validate reports why the device or MPCkit cannot use the file, or nil.
// Accepted: uncompressed integer PCM (8, 16, 24, 32-bit) or 32-bit float,
// mono or stereo, at most 20 minutes.
func (i Info) Validate() error {
	switch i.Format {
	case FormatPCM:
		switch i.Bits {
		case 8, 16, 24, 32:
		default:
			return fmt.Errorf("%d-bit PCM is not supported (use 8, 16, 24 or 32-bit)", i.Bits)
		}
	case FormatFloat:
		if i.Bits != 32 {
			return fmt.Errorf("%d-bit float is not supported (use 32-bit float)", i.Bits)
		}
	default:
		return fmt.Errorf("compressed WAV (format code %#x) is not supported; use uncompressed PCM or float", i.Format)
	}
	if i.Channels > MaxChannels {
		return fmt.Errorf("%d channels: only mono and stereo are supported", i.Channels)
	}
	if i.Frames == 0 {
		return errors.New("WAV file holds no audio")
	}
	if i.Duration() > MaxDuration {
		return fmt.Errorf("sample is %s long; the limit is 20 minutes", i.Duration().Round(time.Second))
	}
	return nil
}
