package project

import (
	"fmt"

	"github.com/bajdev/MPCkit/internal/xpj"
)

// checkBank refuses a bank number outside 0 (A) to 7 (H).
func checkBank(bank int) error {
	if bank < 0 || bank >= Pads/PadsPerBank {
		return fmt.Errorf("bank %d out of range 0-%d", bank, Pads/PadsPerBank-1)
	}
	return nil
}

// ClearBank clears every pad of bank (0 for A to 7 for H) as ClearPad does,
// dropping pool entries no pad uses any more. Clearing a pad cannot fail, so
// once the bank number is checked the whole bank is cleared. An empty bank
// changes nothing.
func (p *Project) ClearBank(bank int) error {
	if err := checkBank(bank); err != nil {
		return err
	}
	first := bank * PadsPerBank
	for slot := first; slot < first+PadsPerBank; slot++ {
		if len(padFiles(p.pad(slot))) > 0 {
			p.clear(slot, "")
			p.dirty = true
		}
	}
	return nil
}

// RemoveSequenceBank removes every sequence in bank (0 for A to 7 for H) as
// RemoveSequence does. A project must keep one sequence, so if the bank holds
// them all, they are replaced by a fresh sequence in the bank's first slot,
// made as AddSequence makes one, and it becomes current. If that fails,
// nothing changes.
func (p *Project) RemoveSequenceBank(bank int) error {
	if err := checkBank(bank); err != nil {
		return err
	}
	var slots []int
	for _, q := range p.Sequences() {
		if q.Slot/PadsPerBank == bank {
			slots = append(slots, q.Slot)
		}
	}
	if len(slots) == 0 {
		return nil
	}
	seqs := p.sequences()
	if len(slots) < len(seqs.Items) {
		// Each sequence exists and others stay, so RemoveSequence cannot
		// refuse any of them.
		for _, slot := range slots {
			if err := p.RemoveSequence(slot); err != nil {
				return err
			}
		}
		return nil
	}
	first := bank * PadsPerBank
	saved := seqs.Items
	seqs.Items = nil
	if err := p.AddSequence(first); err != nil {
		seqs.Items = saved
		return err
	}
	if p.data.Lookup("currentSequence") != nil {
		_ = p.data.SetKey("currentSequence", xpj.NewInt(int64(first)))
	}
	return nil
}
