// Command checkpoint-e builds the projects for device check D1 part 2
// (docs/sequence-drag-plan.md): DemoSeqMove with sequences moved, swapped
// or copied by changing only their keys, and in some projects the song's
// items and currentSequence. Run it from src/; it works the same in cmd,
// PowerShell and sh:
//
//	go run ./tools/checkpoint-e -out E:\
//
// Each project is written as a folder export into -out.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bajdev/MPCkit/internal/export"
	"github.com/bajdev/MPCkit/internal/project"
	"github.com/bajdev/MPCkit/internal/xpj"
)

// Slots in DemoSeqMove: A1 "One", A2 "Two" (with notes), A3 "Three"
// (current), and B5, the device's copy of A2. The song plays slots
// 0, 1, 2, 1.
const (
	slotOne   = 0  // A1
	slotTwo   = 1  // A2
	slotThree = 2  // A3
	slotCopy  = 20 // B5
	slotC9    = 40
	slotH16   = 127
)

// songIndex is the key of a song item that names a sequence.
const songIndex = "item.sequenceIndex"

func main() {
	out := flag.String("out", "", "write the projects into this `folder` (e.g. the device's card)")
	force := flag.Bool("force", false, "replace projects already in -out")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: checkpoint-e -out FOLDER [-force]")
		os.Exit(2)
	}
	if err := run(*out, *force); err != nil {
		fmt.Fprintln(os.Stderr, "checkpoint-e:", err)
		os.Exit(1)
	}
}

func run(out string, force bool) error {
	src := filepath.Join("testdata", "device", "DemoSeqMove.xpj")
	if _, err := os.Stat(src); err != nil {
		return errors.New("run this from src/ (testdata/device/DemoSeqMove.xpj not found)")
	}
	for _, c := range []struct {
		name string
		edit func(d *xpj.Node) error
	}{
		// Move A2 to C9; the song is left as it is. If song items name
		// slots, the song loses "Two"; if they name positions, it changes.
		{"MpckitQ1", func(d *xpj.Node) error {
			return setKey(d, slotTwo, slotC9)
		}},
		// The same move, with the song's items following the sequence.
		{"MpckitQ2", func(d *xpj.Node) error {
			if err := setKey(d, slotTwo, slotC9); err != nil {
				return err
			}
			return mapSong(d, map[int64]int64{slotTwo: slotC9})
		}},
		// Swap A1 and A3; the song and the current sequence follow them.
		{"MpckitQ3", func(d *xpj.Node) error {
			if err := swapKeys(d, slotOne, slotThree); err != nil {
				return err
			}
			if err := mapSong(d, map[int64]int64{slotOne: slotThree, slotThree: slotOne}); err != nil {
				return err
			}
			return d.SetKey("currentSequence", xpj.NewInt(slotOne))
		}},
		// Copy A2 to H16, first in sequences[] as the device puts its copy.
		{"MpckitQ4", func(d *xpj.Node) error {
			return copySeq(d, slotTwo, slotH16)
		}},
	} {
		p, _, err := project.Load(src)
		if err != nil {
			return err
		}
		if err := p.SetName(c.name); err != nil {
			return err
		}
		// Tell the device's copy apart from A2 on the song screen.
		if err := p.SetSequence(slotCopy, "Two B5", 110, 4); err != nil {
			return err
		}
		d, err := p.File().Data()
		if err != nil {
			return err
		}
		if err := c.edit(d); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		path, err := export.Folder(p, out, force)
		var exists *export.ExistsError
		if errors.As(err, &exists) {
			return fmt.Errorf("%w (use -force to replace)", err)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		fmt.Println("wrote", path)
	}
	return nil
}

// entry returns the sequences[] entry whose key is slot.
func entry(d *xpj.Node, slot int64) (*xpj.Node, error) {
	seqs, err := d.Get("sequences")
	if err != nil {
		return nil, err
	}
	for _, s := range seqs.Items {
		if k, err := s.Lookup("key").Int(); err == nil && k == slot {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no sequence in slot %d", slot)
}

func setKey(d *xpj.Node, from, to int64) error {
	s, err := entry(d, from)
	if err != nil {
		return err
	}
	return s.SetKey("key", xpj.NewInt(to))
}

func swapKeys(d *xpj.Node, a, b int64) error {
	sa, err := entry(d, a)
	if err != nil {
		return err
	}
	sb, err := entry(d, b)
	if err != nil {
		return err
	}
	if err := sa.SetKey("key", xpj.NewInt(b)); err != nil {
		return err
	}
	return sb.SetKey("key", xpj.NewInt(a))
}

func copySeq(d *xpj.Node, from, to int64) error {
	s, err := entry(d, from)
	if err != nil {
		return err
	}
	c := s.Clone()
	if err := c.SetKey("key", xpj.NewInt(to)); err != nil {
		return err
	}
	seqs, err := d.Get("sequences")
	if err != nil {
		return err
	}
	return seqs.Insert(0, c)
}

// mapSong rewrites every song item's sequence index found in m.
func mapSong(d *xpj.Node, m map[int64]int64) error {
	songs, err := d.Get("songs")
	if err != nil {
		return err
	}
	for _, song := range songs.Items {
		items := song.Lookup("items")
		if items == nil {
			continue
		}
		for _, it := range items.Items {
			v, err := it.Lookup(songIndex).Int()
			if err != nil {
				return err
			}
			if to, ok := m[v]; ok {
				if err := it.SetKey(songIndex, xpj.NewInt(to)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
