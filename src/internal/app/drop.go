package app

import (
	"fmt"
	"strings"

	"github.com/bajdev/MPCkit/internal/project"
)

// Several WAV files or a folder dropped on an empty bank fill it
// (docs/multi-drop-plan.md). The paths come from main.go's drop handler;
// DropOnBank is on Host, so the window cannot pass a path.

// NoWAVs is the message when nothing usable was dropped, and ErrNoWAVs the
// error that carries it.
const NoWAVs = "Only WAV files can be assigned to a pad."

var ErrNoWAVs error = sentence(NoWAVs)

// skippedShown is how many unusable files a message names; the rest are
// counted.
const skippedShown = 5

// DropOnBank fills an empty bank (0 for A) with the WAV files among paths,
// in name order. It returns the new state and the notice for the window
// ("Assigned 16 files to C1-C16. 4 more were left out."). An error is for
// the error banner: the drop was refused, nothing could be assigned, or,
// alongside a notice, some files could not be used.
func (h *Host) DropOnBank(bank int, paths []string) (State, string, error) {
	// Listing folders reads the disk, so it runs before taking the lock.
	files, _ := project.GatherWAVs(paths)
	if len(files) == 0 {
		return h.s.State(), "", ErrNoWAVs
	}
	var res project.FillResult
	st, err := h.s.apply(func(p *project.Project) (err error) {
		res, err = p.FillBank(bank, files)
		return err
	})
	if err != nil {
		return st, "", err
	}
	notice, problems := FillMessage(bank, res)
	if problems != "" {
		return st, notice, sentence(problems)
	}
	return st, notice, nil
}

// FillMessage describes a fill (decision 10). notice is one line saying what
// was assigned, empty if nothing was. problems lists what went wrong: the
// files that could not be used, and, when nothing was assigned, why.
func FillMessage(bank int, res project.FillResult) (notice, problems string) {
	var lines []string
	if n := len(res.Slots); n > 0 {
		where := project.SlotName(res.Slots[0])
		if n > 1 {
			where += "-" + project.SlotName(res.Slots[n-1])
		}
		notice = fmt.Sprintf("Assigned %s to %s.", count(n, "file", "files"), where)
		switch {
		case res.Full:
			notice += fmt.Sprintf(" The project has %d samples, the most the device allows", project.MaxSamples)
			if res.LeftOut > 0 {
				notice += fmt.Sprintf(": %s left out.", count(res.LeftOut, "more was", "more were"))
			} else {
				notice += "."
			}
		case res.LeftOut > 0:
			notice += fmt.Sprintf(" %s left out.", count(res.LeftOut, "more was", "more were"))
		}
	} else {
		line := fmt.Sprintf("No files were assigned to bank %c.", 'A'+bank)
		if res.Full {
			line += fmt.Sprintf(" The project already has %d samples, the most the device allows.", project.MaxSamples)
		}
		lines = append(lines, line)
	}
	if k := len(res.Skipped); k > 0 {
		lines = append(lines, count(k, "file", "files")+" could not be used:")
		for _, s := range res.Skipped[:min(k, skippedShown)] {
			lines = append(lines, s.Reason)
		}
		if k > skippedShown {
			lines = append(lines, fmt.Sprintf("and %d more", k-skippedShown))
		}
	}
	return notice, strings.Join(lines, "\n")
}

// sentence is an error whose text is shown to the user as it is: whole
// sentences, one per line.
type sentence string

func (s sentence) Error() string { return string(s) }

// count writes n with the singular or plural words after it.
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
