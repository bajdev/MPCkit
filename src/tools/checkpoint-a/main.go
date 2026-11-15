// Command checkpoint-a builds the projects for device checkpoint A (build
// plan section 7) with the Go writer, and zips them. Run it from the repo
// root; it works the same in cmd, PowerShell and sh:
//
//	go run ./tools/checkpoint-a
//	go run ./tools/checkpoint-a -zip pack.zip -folder E:\
//
// -folder also writes the projects unzipped, e.g. straight onto the card.
package main

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"git.monoexp.com/bajdev/mpckit/internal/export"
	"git.monoexp.com/bajdev/mpckit/internal/project"
)

func main() {
	zipPath := flag.String("zip", "mpckit-checkpoint-a.zip", "write the pack to this zip `file`")
	folder := flag.String("folder", "", "also write the projects into this `folder` (e.g. the device's card)")
	force := flag.Bool("force", false, "replace projects already in -folder")
	flag.Parse()
	if err := run(*zipPath, *folder, *force); err != nil {
		fmt.Fprintln(os.Stderr, "checkpoint-a:", err)
		os.Exit(1)
	}
}

func run(zipPath, folder string, force bool) error {
	src := filepath.Join("testdata", "device", "DemoFile"+project.DataSuffix)
	if _, err := os.Stat(src); err != nil {
		return errors.New("run this from the mpckit repo root (testdata/device not found)")
	}
	work, err := os.MkdirTemp("", "mpckit-checkpoint-a-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	projects, err := build(src, work)
	if err != nil {
		return err
	}
	pack := filepath.Join(work, "pack")
	if err := os.Mkdir(pack, 0o755); err != nil {
		return err
	}
	for _, p := range projects {
		if _, err := export.Folder(p, pack, false); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
		if folder != "" {
			path, err := export.Folder(p, folder, force)
			var exists *export.ExistsError
			if errors.As(err, &exists) {
				return fmt.Errorf("%w (use -force to replace)", err)
			}
			if err != nil {
				return fmt.Errorf("%s: %w", p.Name, err)
			}
			fmt.Println("wrote", path)
		}
		fmt.Printf("%s: %d samples, %d sequences\n", p.Name, p.SampleCount(), len(p.Sequences()))
	}
	if err := zipDir(pack, zipPath); err != nil {
		return err
	}
	fmt.Println("wrote", zipPath)
	return nil
}

type pad struct {
	label  string
	file   string
	coarse int
	fine   int
	mute   int
}

type seq struct {
	name string
	bpm  float64
	bars int
}

// spec is one test project: pads (applied in order) and sequences 1, 2, ...
type spec struct {
	name string
	pads []pad
	seqs []seq
}

func build(src, work string) ([]*project.Project, error) {
	f := func(name string) string { return filepath.Join(src, name) }
	kick, snare := f("80s-Kik-Dreamer2 SA.wav"), f("80s-Snr-Dreamer SA.wav")
	hat1, hat2 := f("80s-Hat-Dreamer1 SA.wav"), f("80s-Hat-Dreamer2 SA.wav")
	bass := f("Bass-Pure C.wav")

	tones := filepath.Join(work, "tones")
	if err := os.Mkdir(tones, 0o755); err != nil {
		return nil, err
	}
	t := func(name string, rate, bits, channels int, ext bool) (string, error) {
		path := filepath.Join(tones, name)
		return path, os.WriteFile(path, tone(rate, bits, channels, ext), 0o644)
	}
	plain, err1 := t("44k 16bit mono.wav", 44100, 16, 1, false)
	ext48, err2 := t("48k 24bit stereo extensible.wav", 48000, 24, 2, true)
	ext44s, err3 := t("44k 16bit stereo extensible.wav", 44100, 16, 2, true)
	ext44m, err4 := t("44k 16bit mono extensible.wav", 44100, 16, 1, true)
	if err := errors.Join(err1, err2, err3, err4); err != nil {
		return nil, err
	}

	specs := []spec{
		{
			// A1 kit, A2 tuning extremes, A3 group-1 choke, A5 1 and 64 bars.
			name: "MpckitKit",
			pads: []pad{
				{label: "A1", file: kick}, {label: "A2", file: snare},
				{label: "A3", file: hat1, mute: 1}, {label: "A4", file: hat2, mute: 1},
				{label: "B1", file: bass},
				{label: "B2", file: bass, coarse: -24}, {label: "B3", file: bass, coarse: 24},
				{label: "B4", file: bass, fine: -90}, {label: "B5", file: bass, fine: 90},
				{label: "B6", file: bass, coarse: -24, fine: -90}, {label: "B7", file: bass, coarse: 24, fine: 90},
			},
			seqs: []seq{{"One Bar", 120, 1}, {"Long", 120, 64}},
		},
		{
			// A3: the lowest and highest mute groups. The 2026-11-10 run
			// used 16 and 32 (see README); MPCkit now refuses 32.
			name: "MpckitMute",
			pads: []pad{
				{label: "A1", file: hat1, mute: 16}, {label: "A2", file: hat2, mute: 16},
				{label: "A3", file: hat1, mute: 1}, {label: "A4", file: hat2, mute: 1},
			},
		},
		{
			// A5: BPM extremes.
			name: "MpckitTempo",
			pads: []pad{{label: "A1", file: kick}},
			seqs: []seq{{"Slow", 30, 2}, {"Fast", 300, 2}, {"Very Slow", 20, 2}, {"Very Fast", 999, 2}},
		},
		{
			// Extensible WAV headers (section 7, "Not yet tested").
			name: "MpckitWav",
			pads: []pad{{label: "A1", file: plain}, {label: "A2", file: ext48}, {label: "A3", file: ext44s}, {label: "A4", file: ext44m}},
		},
	}

	var out []*project.Project
	for _, s := range specs {
		p, err := s.build()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.name, err)
		}
		out = append(out, p)
	}
	return out, nil
}

func (s spec) build() (*project.Project, error) {
	p, err := project.New(s.name)
	if err != nil {
		return nil, err
	}
	for _, pd := range s.pads {
		slot := int(pd.label[0]-'A')*16 + atoi(pd.label[1:]) - 1
		if err := p.AssignSample(slot, pd.file); err != nil {
			return nil, err
		}
		if pd.coarse != 0 || pd.fine != 0 {
			if err := p.SetTune(slot, pd.coarse, pd.fine); err != nil {
				return nil, err
			}
		}
		if pd.mute != 0 {
			if err := p.SetMuteGroup(slot, pd.mute); err != nil {
				return nil, err
			}
		}
	}
	for i, sq := range s.seqs {
		if i > 0 {
			if err := p.AddSequence(i); err != nil {
				return nil, err
			}
		}
		if err := p.SetSequence(i, sq.name, sq.bpm, sq.bars); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// zipDir zips the contents of dir with forward-slash entry names, folders
// first, in name order.
func zipDir(dir, path string) error {
	var entries []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			rel += "/"
		}
		entries = append(entries, rel)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(entries)

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	now := time.Now()
	for _, name := range entries {
		h := &zip.FileHeader{Name: name, Modified: now, Method: zip.Deflate}
		if name[len(name)-1] == '/' {
			h.Method = zip.Store
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			out.Close()
			return err
		}
		if h.Method == zip.Store {
			continue
		}
		in, err := os.Open(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(w, in)
		in.Close()
		if err != nil {
			out.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
