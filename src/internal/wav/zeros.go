package wav

// ZeroCrossings returns, in order, the frames where the sample crosses
// zero, for the start/end editor's "Snap to zero" (waveform plan, PR W4b).
// Channels are mixed to mono by averaging them. Every frame that is exactly
// zero counts. Where the mix changes sign between two frames, the one of
// the pair closer to zero counts, the first on a tie.
func ZeroCrossings(chans [][]float32) []uint32 {
	if len(chans) == 0 {
		return nil
	}
	n := len(chans[0])
	mono := make([]float32, n)
	for _, ch := range chans {
		for i := 0; i < n && i < len(ch); i++ {
			mono[i] += ch[i]
		}
	}
	scale := float32(len(chans))
	for i := range mono {
		mono[i] /= scale
	}
	var out []uint32
	add := func(i int) {
		if k := len(out); k == 0 || out[k-1] != uint32(i) {
			out = append(out, uint32(i))
		}
	}
	for i, v := range mono {
		if v == 0 {
			add(i)
			continue
		}
		if i+1 < n {
			w := mono[i+1]
			if w != 0 && (v < 0) != (w < 0) {
				if abs(v) <= abs(w) {
					add(i)
				} else {
					add(i + 1)
				}
			}
		}
	}
	return out
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
