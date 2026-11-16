package app

import (
	"encoding/binary"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/bajdev/MPCkit/internal/project"
	"github.com/bajdev/MPCkit/internal/wav"
)

// AudioPrefix is the URL path the preview loads samples from:
// /audio/<slot> serves the pad's source WAV as it is, and
// /audio/<slot>?pcm=1 serves it decoded, for formats the webview cannot
// decode (build plan 4.5 and section 9).
const AudioPrefix = "/audio/"

// Headers on the plain WAV answer giving the file's own frame count and
// sample rate. The webview decodes to the sound output's rate, but a pad's
// start and end are frames of the file, so the preview needs these to turn
// them into seconds.
const (
	FramesHeader     = "X-Mpckit-Frames"
	SampleRateHeader = "X-Mpckit-Sample-Rate"
)

// AudioMiddleware serves AudioPrefix and passes every other request on.
// It is for main.go's asset server.
func (h *Host) AudioMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, AudioPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		h.s.serveAudio(w, r)
	})
}

func (s *Service) serveAudio(w http.ResponseWriter, r *http.Request) {
	slot, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, AudioPrefix))
	if err != nil || slot < 0 || slot >= project.Pads {
		http.NotFound(w, r)
		return
	}
	// Unlock by defer, so a panic cannot leave the lock held.
	v, src, err := func() (project.Pad, string, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		v, err := s.p.Pad(slot)
		src, _ := s.p.Source(v.File)
		return v, src, err
	}()
	if err != nil || v.Empty() || src == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	if r.URL.Query().Get("pcm") == "" {
		info, err := wav.ReadFile(src)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set(FramesHeader, strconv.FormatInt(info.Frames, 10))
		w.Header().Set(SampleRateHeader, strconv.Itoa(info.SampleRate))
		w.Header().Set("Content-Type", "audio/wav")
		http.ServeFile(w, r, src)
		return
	}

	// Decoded: a 12-byte header (sample rate, channels, frames as uint32
	// little-endian), then each channel's frames as float32 little-endian.
	info, chans, err := wav.DecodeFile(src)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	frames := len(chans[0])
	buf := make([]byte, 12+4*frames*len(chans))
	binary.LittleEndian.PutUint32(buf[0:], uint32(info.SampleRate))
	binary.LittleEndian.PutUint32(buf[4:], uint32(len(chans)))
	binary.LittleEndian.PutUint32(buf[8:], uint32(frames))
	off := 12
	for _, ch := range chans {
		for _, v := range ch {
			binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(v))
			off += 4
		}
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(buf)
}
