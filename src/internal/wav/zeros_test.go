package wav

import (
	"slices"
	"testing"
)

func TestZeroCrossings(t *testing.T) {
	for _, c := range []struct {
		name  string
		chans [][]float32
		want  []uint32
	}{
		{"none", nil, nil},
		{"no crossing", [][]float32{{0.5, 0.4, 0.3}}, nil},
		{"offset only", [][]float32{{-0.2, -0.1, -0.3}}, nil},
		// 0.5 → -0.1: frame 1 is closer to zero.
		{"sign change, second closer", [][]float32{{0.5, -0.1, -0.4}}, []uint32{1}},
		// 0.1 → -0.5: frame 0 is closer.
		{"sign change, first closer", [][]float32{{0.1, -0.5, -0.4}}, []uint32{0}},
		{"tie takes the first", [][]float32{{0.2, -0.2}}, []uint32{0}},
		// Exact zeros count, silence included, and are not repeated.
		{"exact zeros", [][]float32{{0.3, 0, -0.3, 0, 0}}, []uint32{1, 3, 4}},
		// Frame 2 closes one crossing (0.4 → -0.1) and opens the next
		// (-0.1 → 0.3): listed once.
		{"shared frame", [][]float32{{0.6, 0.4, -0.1, 0.3}}, []uint32{2}},
		// Left and right cancel at frame 1; the mix crosses between 2 and 3.
		{"stereo is mixed", [][]float32{{0.5, 0.5, 0.4, -0.6}, {0.5, -0.5, 0.2, -0.2}}, []uint32{1, 2}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := ZeroCrossings(c.chans); !slices.Equal(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// A square wave flipping every 50 frames crosses zero at each flip. The
// two frames around a flip are equally far from zero, so the first counts.
func TestZeroCrossingsSquare(t *testing.T) {
	const n = 1000
	ch := make([]float32, n)
	for i := range ch {
		if (i/50)%2 == 0 {
			ch[i] = 0.5
		} else {
			ch[i] = -0.5
		}
	}
	got := ZeroCrossings([][]float32{ch})
	if len(got) != n/50-1 {
		t.Fatalf("%d crossings, want %d", len(got), n/50-1)
	}
	for k, f := range got {
		if want := uint32(49 + 50*k); f != want {
			t.Errorf("crossing %d at %d, want %d", k, f, want)
		}
	}
}
