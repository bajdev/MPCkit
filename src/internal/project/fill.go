package project

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// Filling an empty bank with several dropped WAV files or a folder of them
// (docs/multi-drop-plan.md). The bank only ever starts empty, so nothing is
// replaced: the files go to pads 1-16 in name order.

// PadsPerBank is the number of pads in a bank, A1-A16 to H1-H16.
const PadsPerBank = 16

// GatherWAVs turns dropped paths into the WAV files to place, in name order
// (CompareNames). A dropped file is used as it is if it is a WAV. For a
// dropped folder, only the WAV files directly in it are used: subfolders,
// hidden files (names starting with a dot) and links are skipped, since the
// user picked the folder, not each file in it. Folders are listed by name
// only; no file is opened here. Everything else, and any path that cannot
// be read, is counted in ignored.
func GatherWAVs(paths []string) (files []string, ignored int) {
	for _, path := range paths {
		info, err := os.Stat(path)
		switch {
		case err != nil:
			ignored++
		case info.IsDir():
			entries, err := os.ReadDir(path)
			if err != nil {
				ignored++
				continue
			}
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), ".") || !e.Type().IsRegular() || !isWAV(e.Name()) {
					ignored++
					continue
				}
				files = append(files, filepath.Join(path, e.Name()))
			}
		case isWAV(path):
			files = append(files, path)
		default:
			ignored++
		}
	}
	slices.SortStableFunc(files, func(a, b string) int {
		return cmp.Or(CompareNames(filepath.Base(a), filepath.Base(b)), strings.Compare(a, b))
	})
	return files, ignored
}

func isWAV(name string) bool { return strings.EqualFold(filepath.Ext(name), ".wav") }

// CompareNames orders file names as a file manager lists them: ignoring
// case and common accents ("Écho" with "echo"), and with runs of digits
// compared as numbers ("kick 2" before "kick 10"). Names equal that way
// ("01" and "1") fall back to a plain comparison, so the order is fixed.
func CompareNames(a, b string) int {
	x, y := []rune(foldName(a)), []rune(foldName(b))
	for len(x) > 0 && len(y) > 0 {
		if isDigit(x[0]) && isDigit(y[0]) {
			nx, ny := digitRun(x), digitRun(y)
			if c := compareNumbers(x[:nx], y[:ny]); c != 0 {
				return c
			}
			x, y = x[nx:], y[ny:]
			continue
		}
		if c := cmp.Compare(x[0], y[0]); c != 0 {
			return c
		}
		x, y = x[1:], y[1:]
	}
	return cmp.Or(cmp.Compare(len(x), len(y)), strings.Compare(a, b))
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func digitRun(r []rune) int {
	n := 0
	for n < len(r) && isDigit(r[n]) {
		n++
	}
	return n
}

// compareNumbers compares two runs of ASCII digits by value, whatever
// their length, without converting them (a name can hold a long number).
func compareNumbers(a, b []rune) int {
	trim := func(r []rune) []rune {
		for len(r) > 1 && r[0] == '0' {
			r = r[1:]
		}
		return r
	}
	a, b = trim(a), trim(b)
	return cmp.Or(cmp.Compare(len(a), len(b)), slices.Compare(a, b))
}

// foldName lowercases a name and replaces common Latin accented letters with
// their base letters, for sorting only. Go's standard library has no
// accent folding, and the plan allows no new modules, hence the table.
func foldName(s string) string {
	var b strings.Builder
	for _, r := range s {
		r = unicode.ToLower(r)
		if base, ok := accentFold[r]; ok {
			b.WriteString(base)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var accentFold = func() map[rune]string {
	m := map[rune]string{}
	for base, letters := range map[string]string{
		"a": "àáâãäåāăą", "c": "çćĉċč", "d": "ďđð", "e": "èéêëēĕėęě",
		"g": "ĝğġģ", "h": "ĥħ", "i": "ìíîïĩīĭįı", "j": "ĵ", "k": "ķ",
		"l": "ĺļľŀł", "n": "ñńņňŉ", "o": "òóôõöøōŏő", "r": "ŕŗř",
		"s": "śŝşš", "t": "ţťŧ", "u": "ùúûüũūŭůűų", "w": "ŵ", "y": "ýÿŷ",
		"z": "źżž", "ae": "æ", "oe": "œ", "ss": "ß", "th": "þ",
	} {
		for _, r := range letters {
			m[r] = base
		}
	}
	return m
}()

// Skipped is a file FillBank could not use, with the reason.
type Skipped struct {
	File   string // path
	Reason string // AssignSample's error, which names the file
}

// FillResult is what FillBank did.
type FillResult struct {
	Slots   []int     // pads filled, in order
	Skipped []Skipped // files that could not be used
	LeftOut int       // files not tried: the bank was full, or the project at MaxSamples
	Full    bool      // assigning stopped at MaxSamples
}

// BankNotEmptyError is FillBank's error for a bank that holds a sample. Its
// text is the sentence the window and the CLI show (decision 2).
type BankNotEmptyError struct{ Bank int }

func (e *BankNotEmptyError) Error() string {
	return fmt.Sprintf("Bank %c is not empty. Drop several files or a folder on an empty bank.", 'A'+e.Bank)
}

// FillBank assigns files, in order, to pads 1-16 of bank (0 for A to 7 for
// H). The bank must be empty; otherwise, or for a bad bank number, nothing
// changes. Each file goes through AssignSample. A file it refuses is
// recorded and the next file tries the same pad. Assigning stops when the
// bank is full or the project reaches MaxSamples; the files not tried are
// counted in LeftOut.
func (p *Project) FillBank(bank int, files []string) (FillResult, error) {
	var res FillResult
	if err := checkBank(bank); err != nil {
		return res, err
	}
	first := bank * PadsPerBank
	for slot := first; slot < first+PadsPerBank; slot++ {
		if len(padFiles(p.pad(slot))) > 0 {
			return res, &BankNotEmptyError{Bank: bank}
		}
	}
	slot := first
	for i, file := range files {
		if slot == first+PadsPerBank {
			res.LeftOut = len(files) - i
			break
		}
		err := p.AssignSample(slot, file)
		switch {
		case errors.Is(err, ErrSampleLimit):
			res.Full = true
			res.LeftOut = len(files) - i
			return res, nil
		case err != nil:
			res.Skipped = append(res.Skipped, Skipped{File: file, Reason: err.Error()})
		default:
			res.Slots = append(res.Slots, slot)
			slot++
		}
	}
	return res, nil
}
