package main

import "testing"

func TestFitSize(t *testing.T) {
	cases := []struct{ areaW, areaH, w, h int }{
		{1920, 1032, 1280, 720}, // 1080p at 100%: capped at the largest size
		{1280, 688, 1100, 619},  // 1080p at 150% (taskbar removed): limited by height
		{1536, 824, 1280, 720},  // 1080p at 125%
		{2560, 1400, 1280, 720},
		{1024, 600, 960, 540}, // too small: the minimum
		{0, 0, 960, 540},
	}
	for _, c := range cases {
		if w, h := fitSize(c.areaW, c.areaH); w != c.w || h != c.h {
			t.Errorf("fitSize(%d, %d) = %d x %d, want %d x %d", c.areaW, c.areaH, w, h, c.w, c.h)
		}
	}
}
