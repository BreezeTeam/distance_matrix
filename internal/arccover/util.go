package arccover

import "sort"

func ceilDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func splitmix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	z := x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// AssignEdgeIDs ensures Arc.ID == index in required.
func AssignEdgeIDs(required []Arc) {
	for i := range required {
		required[i].ID = EdgeID(i)
	}
}

// NormalizeRequired copies arcs, sorts by (From, To), and assigns sequential IDs.
func NormalizeRequired(required []Arc) []Arc {
	out := make([]Arc, len(required))
	copy(out, required)
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].To != out[j].To {
			return out[i].To < out[j].To
		}
		return out[i].ID < out[j].ID
	})
	AssignEdgeIDs(out)
	return out
}
