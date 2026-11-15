package main

import "testing"

func TestParseSlot(t *testing.T) {
	for in, want := range map[string]int{"A1": 0, "a5": 4, "B1": 16, "h16": 127, " C3 ": 34} {
		if got, err := parseSlot(in); err != nil || got != want {
			t.Errorf("parseSlot(%q) = %d, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "A", "A0", "A17", "I1", "1A", "Ax"} {
		if _, err := parseSlot(in); err == nil {
			t.Errorf("parseSlot(%q) succeeded", in)
		}
	}
	if from, to, err := parsePair("A1:b5"); err != nil || from != 0 || to != 20 {
		t.Errorf("parsePair = %d %d %v", from, to, err)
	}
	if _, err := parseSeqNum("129"); err == nil {
		t.Error("sequence 129 accepted")
	}
}
