// Package app is the Wails service the frontend calls. It owns the current
// project and returns a fresh State after every call; the frontend keeps no
// project data of its own (build plan 4.6 and 5).
package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bajdev/MPCkit/internal/export"
	"github.com/bajdev/MPCkit/internal/project"
)

// Version and Commit are set by production builds (Taskfile.yml) with
// -ldflags "-X github.com/bajdev/MPCkit/internal/app.Version=0.5.5
// -X github.com/bajdev/MPCkit/internal/app.Commit=2354f57".
var (
	Version = "dev"
	Commit  = ""
)

// DefaultName is the name of a new project.
const DefaultName = "NewKit"

// Pad is one pad as the frontend shows it.
type Pad struct {
	Slot      int    `json:"slot"`
	Label     string `json:"label"` // "A1" .. "H16"
	Name      string `json:"name"`  // sample name; empty for an empty pad
	File      string `json:"file"`  // pool file name
	Coarse    int    `json:"coarse"`
	Fine      int    `json:"fine"`
	MuteGroup int    `json:"muteGroup"`
	HasNotes  bool   `json:"hasNotes"`
	// Audio is the URL the preview loads the sample from; empty for an
	// empty pad. It changes when the pad's source file changes.
	Audio string `json:"audio"`
	// Start and End are the slice's first and last frame. The preview
	// plays only this region, as the device does.
	Start int64 `json:"start"`
	End   int64 `json:"end"`
	// Source is where the sample is copied from at export; Missing is set
	// when that file no longer exists.
	Source  string `json:"source"`
	Missing bool   `json:"missing"`
}

// Sequence is one existing sequence.
type Sequence struct {
	Slot     int     `json:"slot"`
	Label    string  `json:"label"` // slot label, "A1" .. "H16"
	Name     string  `json:"name"`
	BPM      float64 `json:"bpm"`
	Bars     int     `json:"bars"`
	HasNotes bool    `json:"hasNotes"`
}

// Limits are the ranges the UI offers, from the device checks.
type Limits struct {
	Pads         int     `json:"pads"`
	MaxSamples   int     `json:"maxSamples"`
	MaxSequences int     `json:"maxSequences"`
	MinCoarse    int     `json:"minCoarse"`
	MaxCoarse    int     `json:"maxCoarse"`
	MinFine      int     `json:"minFine"`
	MaxFine      int     `json:"maxFine"`
	MaxMuteGroup int     `json:"maxMuteGroup"`
	MinBPM       float64 `json:"minBPM"`
	MaxBPM       float64 `json:"maxBPM"`
	MaxBars      int     `json:"maxBars"`
}

// State is everything the frontend shows about the project.
type State struct {
	Name          string     `json:"name"`
	Dirty         bool       `json:"dirty"`
	Pads          []Pad      `json:"pads"`      // all 128, in slot order
	Sequences     []Sequence `json:"sequences"` // existing ones, in slot order
	SampleCount   int        `json:"sampleCount"`
	MissingCount  int        `json:"missingCount"` // pool files whose source is missing
	SequenceCount int        `json:"sequenceCount"`
	Limits        Limits     `json:"limits"`
}

// About is the text for the About dialog.
type About struct {
	Version string `json:"version"`
	Commit  string `json:"commit"` // empty in dev builds
	License string `json:"license"`
	Repo    string `json:"repo"`
}

// LoadResult is what Load returns. Path is empty if the dialog was
// cancelled. Warnings describe differences from the tested firmware.
type LoadResult struct {
	Path     string   `json:"path"`
	State    State    `json:"state"`
	Warnings []string `json:"warnings"`
}

// ExportResult is what an export call returns. If Exists is not empty,
// nothing was written: the frontend asks the user, then calls again with
// overwrite set.
type ExportResult struct {
	Path   string   `json:"path"`
	Exists []string `json:"exists"`
}

// Dialogs are the native file dialogs. Each returns "" when the user
// cancels. main.go implements them with Wails; tests use a fake.
type Dialogs interface {
	OpenWAV() (string, error)
	OpenProject() (string, error)
	ChooseFolder() (string, error)
	SaveZip(defaultName string) (string, error)
}

// Service is the type main.go binds. Wails registers every exported method
// of a bound type by reflection, whatever the comments say, so the window's
// scripts can call each one by name. Service therefore has only the methods
// the frontend needs, none of which takes a file path; TestBoundMethods
// keeps that list on purpose. Go-only methods are on Host.
type Service struct {
	mu      sync.Mutex
	p       *project.Project
	dialogs Dialogs
	closer  func() // closes the window without asking; set by main.go

	// Where the next export goes, as chosen in the dialog. The frontend
	// never passes a path itself, so a script in the window cannot make
	// MPCkit write (or, with overwrite, delete) anywhere else.
	exportDir, zipPath string
}

// Host gives main.go what the window must not reach: loading and
// assigning by path, the dirty flag, the window closer and the audio
// server. It shares the Service's state but is not bound.
type Host struct{ s *Service }

// SetCloser sets what CloseWindow calls.
func (h *Host) SetCloser(f func()) { h.s.closer = f }

// Dirty reports whether the project has changes not yet exported.
func (h *Host) Dirty() bool {
	h.s.mu.Lock()
	defer h.s.mu.Unlock()
	return h.s.p.Dirty()
}

// LoadPath opens the project at path in place of the current one.
func (h *Host) LoadPath(path string) (LoadResult, error) { return h.s.loadPath(path) }

// AssignSamplePath puts the WAV at path on the pad in slot. The file drop
// handler uses it.
func (h *Host) AssignSamplePath(slot int, path string) (State, error) {
	return h.s.assignSample(slot, path)
}

// CloseWindow closes the app without asking about unsaved changes. The
// frontend calls it after the user chose to discard them.
func (s *Service) CloseWindow() {
	if s.closer != nil {
		s.closer()
	}
}

// NewService returns a service holding a new, empty project, and the Host
// that shares it. Bind only the Service.
func NewService(d Dialogs) (*Service, *Host) {
	s := &Service{dialogs: d}
	s.reset()
	return s, &Host{s: s}
}

func (s *Service) reset() {
	p, err := project.New(DefaultName)
	if err != nil {
		panic(err) // the embedded template and a constant name
	}
	s.use(p)
}

// use makes p the current project and forgets the export targets chosen
// for the previous one. Callers hold s.mu.
func (s *Service) use(p *project.Project) {
	s.p = p
	s.exportDir, s.zipPath = "", ""
}

// New replaces the current project with a new, empty one.
func (s *Service) New() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reset()
	return s.state()
}

// State returns the current project's state.
func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state()
}

// About returns the About dialog text.
func (s *Service) About() About {
	return About{
		Version: Version,
		Commit:  Commit,
		License: "MIT License, copyright (c) 2026 bajdev",
		Repo:    "https://github.com/bajdev/MPCkit",
	}
}

// state builds what the frontend shows about the project. Callers hold s.mu.
func (s *Service) state() State {
	proj := s.p
	missing := missingSources(proj)
	st := State{
		Name:         proj.Name,
		Dirty:        proj.Dirty(),
		Pads:         make([]Pad, 0, project.Pads),
		Sequences:    sequenceStates(proj),
		SampleCount:  proj.SampleCount(),
		MissingCount: len(missing),
		Limits:       limits(),
	}
	for slot := 0; slot < project.Pads; slot++ {
		st.Pads = append(st.Pads, padState(proj, slot, missing))
	}
	st.SequenceCount = len(st.Sequences)
	return st
}

// limits returns the device and format limits the frontend enforces.
func limits() Limits {
	return Limits{
		Pads:         project.Pads,
		MaxSamples:   project.MaxSamples,
		MaxSequences: project.MaxSequences,
		MinCoarse:    project.MinCoarse,
		MaxCoarse:    project.MaxCoarse,
		MinFine:      project.MinFine,
		MaxFine:      project.MaxFine,
		MaxMuteGroup: project.MaxMuteGroup,
		MinBPM:       project.MinBPM,
		MaxBPM:       project.MaxBPM,
		MaxBars:      project.MaxBars,
	}
}

// missingSources returns the pool files that export could not copy: no
// source recorded, or one export refuses (see export.Usable).
func missingSources(proj *project.Project) map[string]bool {
	missing := map[string]bool{}
	for _, file := range proj.PoolFiles() {
		if src, _ := proj.Source(file); src == "" || !export.Usable(src) {
			missing[file] = true
		}
	}
	return missing
}

// padState describes the pad in slot. missing is from missingSources.
func padState(proj *project.Project, slot int, missing map[string]bool) Pad {
	view, err := proj.Pad(slot)
	if err != nil {
		// A loaded file with an odd pad; show it as empty.
		return Pad{Slot: slot, Label: project.SlotName(slot)}
	}
	pad := Pad{
		Slot:      slot,
		Label:     project.SlotName(slot),
		Name:      view.SampleName,
		File:      view.File,
		Coarse:    view.Coarse,
		Fine:      view.Fine,
		MuteGroup: view.MuteGroup,
		Start:     view.Start,
		End:       view.End,
	}
	if view.Empty() {
		return pad
	}
	pad.HasNotes = proj.PadHasNotes(slot)
	pad.Source, _ = proj.Source(view.File)
	pad.Missing = missing[view.File]
	if !pad.Missing {
		pad.Audio = audioURL(slot, pad.Source)
	}
	return pad
}

// sequenceStates describes the project's sequences, in slot order.
func sequenceStates(proj *project.Project) []Sequence {
	seqs := []Sequence{}
	for _, seq := range proj.Sequences() {
		seqs = append(seqs, Sequence{
			Slot:     seq.Slot,
			Label:    project.SlotName(seq.Slot),
			Name:     seq.Name,
			BPM:      seq.BPM,
			Bars:     seq.Bars,
			HasNotes: seq.HasNotes,
		})
	}
	return seqs
}

// audioURL is the preview URL for a pad. The key in the query changes with
// the source file, so the frontend's cache never plays a stale sample.
func audioURL(slot int, src string) string {
	sum := sha256.Sum256([]byte(src))
	return fmt.Sprintf("%s%d?k=%s", AudioPrefix, slot, hex.EncodeToString(sum[:6]))
}

// apply runs fn on the project under the lock and returns the new state.
// The state is returned even on error, so the frontend stays in step.
func (s *Service) apply(fn func(p *project.Project) error) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := fn(s.p)
	return s.state(), err
}

// SetName renames the project.
func (s *Service) SetName(name string) (State, error) {
	return s.apply(func(p *project.Project) error { return p.SetName(strings.TrimSpace(name)) })
}

// AssignSampleDialog asks for a WAV file and puts it on the pad in slot.
// Cancelling the dialog changes nothing.
func (s *Service) AssignSampleDialog(slot int) (State, error) {
	path, err := s.dialogs.OpenWAV()
	if err != nil || path == "" {
		return s.State(), err
	}
	return s.assignSample(slot, path)
}

func (s *Service) assignSample(slot int, path string) (State, error) {
	return s.apply(func(p *project.Project) error { return p.AssignSample(slot, path) })
}

// ClearPad empties the pad in slot.
func (s *Service) ClearPad(slot int) (State, error) {
	return s.apply(func(p *project.Project) error { return p.ClearPad(slot) })
}

// SetTune sets a pad's tuning in semitones and cents.
func (s *Service) SetTune(slot, coarse, fine int) (State, error) {
	return s.apply(func(p *project.Project) error { return p.SetTune(slot, coarse, fine) })
}

// SetTrim sets where a pad's sample starts and ends, in frames of the
// sample file.
func (s *Service) SetTrim(slot int, start, end int64) (State, error) {
	return s.apply(func(p *project.Project) error { return p.SetTrim(slot, start, end) })
}

// SetMuteGroup sets a pad's mute group; 0 is off.
func (s *Service) SetMuteGroup(slot, group int) (State, error) {
	return s.apply(func(p *project.Project) error { return p.SetMuteGroup(slot, group) })
}

// ChooseExportFolder asks for the folder to export into and remembers it
// for ExportFolder. It returns the folder, or "" if cancelled.
func (s *Service) ChooseExportFolder() (string, error) {
	dir, err := s.dialogs.ChooseFolder()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exportDir = ""
	if err == nil {
		s.exportDir = dir
	}
	return dir, err
}

// ChooseZipPath asks where to save the zip and remembers it for ExportZip.
// It returns the path, or "" if cancelled.
func (s *Service) ChooseZipPath() (string, error) {
	s.mu.Lock()
	name := s.p.Name
	s.mu.Unlock()
	path, err := s.dialogs.SaveZip(name + ".zip")
	if path != "" && !strings.EqualFold(filepath.Ext(path), ".zip") {
		path += ".zip"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.zipPath = ""
	if err == nil {
		s.zipPath = path
	}
	return path, err
}

// errNoTarget is returned by an export with no target chosen.
var errNoTarget = errors.New("choose where to export first")

// ExportFolder writes the project into the folder ChooseExportFolder
// chose. If the project is already there and overwrite is false, nothing
// is written and Exists lists it.
func (s *Service) ExportFolder(overwrite bool) (ExportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exportDir == "" {
		return ExportResult{}, errNoTarget
	}
	path, err := export.Folder(s.p, s.exportDir, overwrite)
	return s.exported(path, err)
}

// ExportZip writes the project as a zip where ChooseZipPath chose. If the
// file exists and overwrite is false, nothing is written and Exists lists it.
func (s *Service) ExportZip(overwrite bool) (ExportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.zipPath == "" {
		return ExportResult{}, errNoTarget
	}
	path := s.zipPath
	err := export.Zip(s.p, path, overwrite)
	return s.exported(path, err)
}

// CancelExport forgets the chosen export targets. The frontend calls it
// when the user declines to replace an existing export, so that a later
// ExportFolder(true) or ExportZip(true) cannot overwrite it without asking.
func (s *Service) CancelExport() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exportDir, s.zipPath = "", ""
}

func (s *Service) exported(path string, err error) (ExportResult, error) {
	var exists *export.ExistsError
	if errors.As(err, &exists) {
		return ExportResult{Exists: exists.Paths}, nil
	}
	if err != nil {
		return ExportResult{}, err
	}
	s.p.MarkExported()
	s.exportDir, s.zipPath = "", "" // the next export asks again
	return ExportResult{Path: path}, nil
}

// MovePad moves the pad in from to slot to, replacing what was there.
// Recorded notes stay on their slots; the frontend warns first.
func (s *Service) MovePad(from, to int) (State, error) {
	return s.apply(func(p *project.Project) error { return p.MovePad(from, to) })
}

// CopyPad copies the pad in from to slot to, replacing what was there.
func (s *Service) CopyPad(from, to int) (State, error) {
	return s.apply(func(p *project.Project) error { return p.CopyPad(from, to) })
}

// SwapPads exchanges two pads.
func (s *Service) SwapPads(a, b int) (State, error) {
	return s.apply(func(p *project.Project) error { return p.SwapPads(a, b) })
}

// Load asks for a project file and opens it in place of the current one.
// The frontend asks about unsaved changes first.
func (s *Service) Load() (LoadResult, error) {
	path, err := s.dialogs.OpenProject()
	if err != nil || path == "" {
		return LoadResult{State: s.State()}, err
	}
	return s.loadPath(path)
}

// loadPath opens the project at path in place of the current one.
func (s *Service) loadPath(path string) (LoadResult, error) {
	p, warnings, err := project.Load(path)
	if err != nil {
		return LoadResult{State: s.State()}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.use(p)
	if warnings == nil {
		warnings = []string{}
	}
	return LoadResult{Path: path, State: s.state(), Warnings: warnings}, nil
}

// AddSequence adds a sequence in an empty slot: "Sequence NN", 2 bars, at
// the BPM of the most recently edited sequence (else 120).
func (s *Service) AddSequence(slot int) (State, error) {
	return s.apply(func(p *project.Project) error { return p.AddSequence(slot) })
}

// RemoveSequence deletes a sequence and its recorded notes. The last
// sequence cannot be removed.
func (s *Service) RemoveSequence(slot int) (State, error) {
	return s.apply(func(p *project.Project) error { return p.RemoveSequence(slot) })
}

// SetSequence sets a sequence's name, BPM and length in bars. Notes past a
// new, shorter end are left in the file.
func (s *Service) SetSequence(slot int, name string, bpm float64, bars int) (State, error) {
	return s.apply(func(p *project.Project) error {
		return p.SetSequence(slot, strings.TrimSpace(name), bpm, bars)
	})
}
