// Command mpckit-cli builds an MPC Sample project from flags, for device
// tests before the app exists (build plan M3). Edits run in the order given.
//
//	go run ./cmd/mpckit-cli -name Kit -out ~/kits \
//	    -pad A1=kick.wav -pad A2=snare.wav -tune A2=-24,0 -mute A3=1 \
//	    -seq 1=Verse,96,8 -seq 2=Chorus,96,4
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bajdev/MPCkit/internal/export"
	"github.com/bajdev/MPCkit/internal/project"
)

// opFunc is one change to the project, made from a flag's value.
type opFunc func(p *project.Project) error

// parseFunc turns a flag's value into an opFunc, or explains what is wrong.
type parseFunc func(arg string) (opFunc, error)

type op struct {
	flag, arg string
	run       opFunc
}

var ops []op

// opFlag is a repeatable flag whose values become ops.
type opFlag struct {
	name  string
	parse parseFunc
}

func (f opFlag) String() string { return "" }
func (f opFlag) Set(arg string) error {
	run, err := f.parse(arg)
	if err != nil {
		return err
	}
	ops = append(ops, op{flag: f.name, arg: arg, run: run})
	return nil
}

func addOp(name, usage string, parse parseFunc) {
	flag.Var(opFlag{name: name, parse: parse}, name, usage)
}

// parseSlot reads a pad label such as A1 or h16.
func parseSlot(s string) (int, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 2 || s[0] < 'A' || s[0] > 'H' {
		return 0, fmt.Errorf("bad pad %q: want A1-H16", s)
	}
	n, err := strconv.Atoi(s[1:])
	if err != nil || n < 1 || n > 16 {
		return 0, fmt.Errorf("bad pad %q: want A1-H16", s)
	}
	return int(s[0]-'A')*16 + n - 1, nil
}

func parsePair(arg string) (int, int, error) {
	a, b, ok := strings.Cut(arg, ":")
	if !ok {
		return 0, 0, fmt.Errorf("want FROM:TO, e.g. A1:B5")
	}
	from, err := parseSlot(a)
	if err != nil {
		return 0, 0, err
	}
	to, err := parseSlot(b)
	return from, to, err
}

func parseSeqNum(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > project.MaxSequences {
		return 0, fmt.Errorf("bad sequence number %q: want 1-%d", s, project.MaxSequences)
	}
	return n - 1, nil
}

func init() {
	for _, f := range []struct {
		name, usage string
		parse       parseFunc
	}{
		{"pad", "assign a WAV: `PAD=FILE` (e.g. A1=kick.wav)", parsePadOp},
		{"tune", "tune a pad: `PAD=SEMITONES,CENTS` (e.g. B1=-24,0)", parseTuneOp},
		{"mute", "set a pad's mute group: `PAD=GROUP` (0 is off)", parseMuteOp},
		{"trim", "set where a pad's sample starts and ends, in frames: `PAD=START:END` (e.g. A1=100:2000)", parseTrimOp},
		{"clear", "clear a pad: `PAD`", parseClearOp},
		{"copy", "copy pads: `FROM:TO`", parseCopyOp},
		{"move", "move pads: `FROM:TO`", parseMoveOp},
		{"swap", "swap pads: `FROM:TO`", parseSwapOp},
		{"seq", "set a sequence, adding it if needed: `N=NAME,BPM,BARS` (N is 1-128)", parseSeqOp},
		{"rmseq", "remove a sequence: `N`", parseRmseqOp},
	} {
		addOp(f.name, f.usage, f.parse)
	}
}

// parsePadOp reads -pad PAD=FILE.
func parsePadOp(arg string) (opFunc, error) {
	label, file, ok := strings.Cut(arg, "=")
	if !ok {
		return nil, errors.New("want PAD=FILE")
	}
	slot, err := parseSlot(label)
	if err != nil {
		return nil, err
	}
	return func(p *project.Project) error { return p.AssignSample(slot, file) }, nil
}

// parseTuneOp reads -tune PAD=SEMITONES,CENTS.
func parseTuneOp(arg string) (opFunc, error) {
	label, val, ok := strings.Cut(arg, "=")
	c, f, ok2 := strings.Cut(val, ",")
	if !ok || !ok2 {
		return nil, errors.New("want PAD=SEMITONES,CENTS")
	}
	slot, err := parseSlot(label)
	if err != nil {
		return nil, err
	}
	coarse, err1 := strconv.Atoi(strings.TrimSpace(c))
	fine, err2 := strconv.Atoi(strings.TrimSpace(f))
	if err1 != nil || err2 != nil {
		return nil, errors.New("semitones and cents must be integers")
	}
	if coarse < project.MinCoarse || coarse > project.MaxCoarse || fine < project.MinFine || fine > project.MaxFine {
		return nil, fmt.Errorf("tuning out of range (%d..%d semitones, %d..%d cents)",
			project.MinCoarse, project.MaxCoarse, project.MinFine, project.MaxFine)
	}
	return func(p *project.Project) error { return p.SetTune(slot, coarse, fine) }, nil
}

// parseMuteOp reads -mute PAD=GROUP.
func parseMuteOp(arg string) (opFunc, error) {
	label, val, ok := strings.Cut(arg, "=")
	if !ok {
		return nil, errors.New("want PAD=GROUP")
	}
	slot, err := parseSlot(label)
	if err != nil {
		return nil, err
	}
	group, err := strconv.Atoi(val)
	if err != nil {
		return nil, errors.New("group must be an integer")
	}
	return func(p *project.Project) error { return p.SetMuteGroup(slot, group) }, nil
}

// parseTrimOp reads -trim PAD=START:END. The range is checked against the
// sample file when the edit runs.
func parseTrimOp(arg string) (opFunc, error) {
	label, val, ok := strings.Cut(arg, "=")
	from, to, ok2 := strings.Cut(val, ":")
	if !ok || !ok2 {
		return nil, errors.New("want PAD=START:END")
	}
	slot, err := parseSlot(label)
	if err != nil {
		return nil, err
	}
	start, err1 := strconv.ParseInt(strings.TrimSpace(from), 10, 64)
	end, err2 := strconv.ParseInt(strings.TrimSpace(to), 10, 64)
	if err1 != nil || err2 != nil {
		return nil, errors.New("start and end must be whole numbers of frames")
	}
	return func(p *project.Project) error { return p.SetTrim(slot, start, end) }, nil
}

// parseClearOp reads -clear PAD.
func parseClearOp(arg string) (opFunc, error) {
	slot, err := parseSlot(arg)
	if err != nil {
		return nil, err
	}
	return func(p *project.Project) error { return p.ClearPad(slot) }, nil
}

// parseCopyOp reads -copy FROM:TO.
func parseCopyOp(arg string) (opFunc, error) {
	from, to, err := parsePair(arg)
	if err != nil {
		return nil, err
	}
	return func(p *project.Project) error { return p.CopyPad(from, to) }, nil
}

// parseMoveOp reads -move FROM:TO. Recorded notes stay on FROM, as in the
// app, so it warns when there are any.
func parseMoveOp(arg string) (opFunc, error) {
	from, to, err := parsePair(arg)
	if err != nil {
		return nil, err
	}
	return func(p *project.Project) error {
		if p.PadHasNotes(from) {
			fmt.Fprintf(os.Stderr, "warning: %s has recorded notes; they stay on %s\n",
				project.SlotName(from), project.SlotName(from))
		}
		return p.MovePad(from, to)
	}, nil
}

// parseSwapOp reads -swap FROM:TO.
func parseSwapOp(arg string) (opFunc, error) {
	from, to, err := parsePair(arg)
	if err != nil {
		return nil, err
	}
	return func(p *project.Project) error { return p.SwapPads(from, to) }, nil
}

// parseSeqOp reads -seq N=NAME,BPM,BARS. The sequence is added first if
// slot N is empty.
func parseSeqOp(arg string) (opFunc, error) {
	num, val, ok := strings.Cut(arg, "=")
	parts := strings.Split(val, ",")
	if !ok || len(parts) != 3 {
		return nil, errors.New("want N=NAME,BPM,BARS")
	}
	slot, err := parseSeqNum(num)
	if err != nil {
		return nil, err
	}
	bpm, err1 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	bars, err2 := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err1 != nil || err2 != nil {
		return nil, errors.New("BPM must be a number and BARS an integer")
	}
	return func(p *project.Project) error {
		exists := false
		for _, seq := range p.Sequences() {
			exists = exists || seq.Slot == slot
		}
		if !exists {
			if err := p.AddSequence(slot); err != nil {
				return err
			}
		}
		return p.SetSequence(slot, parts[0], bpm, bars)
	}, nil
}

// parseRmseqOp reads -rmseq N.
func parseRmseqOp(arg string) (opFunc, error) {
	slot, err := parseSeqNum(arg)
	if err != nil {
		return nil, err
	}
	return func(p *project.Project) error { return p.RemoveSequence(slot) }, nil
}

func main() {
	load := flag.String("load", "", "start from an existing `project.xpj` instead of a blank project")
	name := flag.String("name", "", "project `name` (default: the loaded project's name)")
	out := flag.String("out", "", "export to this `folder`")
	zipPath := flag.String("zip", "", "export to this zip `file`")
	force := flag.Bool("force", false, "replace an existing export")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: mpckit-cli [-load FILE] -name NAME (-out DIR | -zip FILE) [edits...]\n\nEdits run in the order given and may repeat.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 0 {
		fail(fmt.Errorf("unexpected argument %q", flag.Arg(0)))
	}
	if (*out == "") == (*zipPath == "") {
		fail(errors.New("give exactly one of -out or -zip"))
	}

	p := openProject(*load, *name)
	for _, o := range ops {
		if err := o.run(p); err != nil {
			fail(fmt.Errorf("-%s %s: %w", o.flag, o.arg, err))
		}
	}
	dest := write(p, *out, *zipPath, *force)
	summary(p)
	fmt.Println("wrote", dest)
}

// openProject loads the project at load, or starts a blank one, and names
// it. It exits on error.
func openProject(load, name string) *project.Project {
	var p *project.Project
	var err error
	if load != "" {
		var warnings []string
		p, warnings, err = project.Load(load)
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}
	} else {
		if name == "" {
			fail(errors.New("-name is required for a new project"))
		}
		p, err = project.New(name)
	}
	if err != nil {
		fail(err)
	}
	if name != "" {
		if err := p.SetName(name); err != nil {
			fail(err)
		}
	}
	return p
}

// write exports p to the folder out or the zip file zipPath, whichever is
// set, and returns where it went. It exits on error.
func write(p *project.Project, out, zipPath string, force bool) string {
	var dest string
	var err error
	if out != "" {
		dest, err = export.Folder(p, out, force)
	} else {
		dest, err = zipPath, export.Zip(p, zipPath, force)
	}
	var exists *export.ExistsError
	if errors.As(err, &exists) {
		fail(fmt.Errorf("%w (use -force to replace)", err))
	}
	if err != nil {
		fail(err)
	}
	return dest
}

func summary(p *project.Project) {
	fmt.Printf("%s: %d samples\n", p.Name, p.SampleCount())
	for slot := 0; slot < project.Pads; slot++ {
		v, _ := p.Pad(slot)
		if v.Empty() {
			continue
		}
		fmt.Printf("  %-4s %s", v.Label, v.File)
		if v.Coarse != 0 || v.Fine != 0 {
			fmt.Printf("  tune %+d st %+d ct", v.Coarse, v.Fine)
		}
		if v.MuteGroup != 0 {
			fmt.Printf("  mute group %d", v.MuteGroup)
		}
		fmt.Println()
	}
	for _, s := range p.Sequences() {
		fmt.Printf("  seq %-3d %s, %g BPM, %d bars\n", s.Slot+1, s.Name, s.BPM, s.Bars)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mpckit-cli:", err)
	os.Exit(1)
}
