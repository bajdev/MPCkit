package main

import (
	"path/filepath"
	"testing"
)

const testdata = "../../testdata"

func TestCompare(t *testing.T) {
	t1 := filepath.Join(testdata, "accepted", "MpckitT1.xpj")
	t2 := filepath.Join(testdata, "accepted", "MpckitT2.xpj")
	t3 := filepath.Join(testdata, "accepted", "MpckitT3.xpj")
	if n, err := compare(t1, t1); err != nil || n != 0 {
		t.Errorf("T1 vs T1: %d, %v", n, err)
	}
	// T2 has the same .xpj as T1 but stripped WAVs: two data files differ.
	if n, err := compare(t1, t2); err != nil || n != 2 {
		t.Errorf("T1 vs T2: %d differences, want 2 (%v)", n, err)
	}
	// T3 shares one pool entry: many differences.
	if n, err := compare(t1, t3); err != nil || n < 3 {
		t.Errorf("T1 vs T3: %d, %v", n, err)
	}
	if _, err := compare(t1, "missing.xpj"); err == nil {
		t.Error("missing file accepted")
	}
}
