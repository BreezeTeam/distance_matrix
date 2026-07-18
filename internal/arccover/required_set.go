package arccover

// RequiredSet is a bitset over directed pairs (from, to) for N vertices.
type RequiredSet struct {
	Bits []uint64
	N    int
}

// NewRequiredSet builds a bitset from required arcs.
func NewRequiredSet(n int, required []Arc) RequiredSet {
	total := n * n
	words := (total + 63) / 64
	s := RequiredSet{
		Bits: make([]uint64, words),
		N:    n,
	}
	for _, e := range required {
		s.Add(e.From, e.To)
	}
	return s
}

func (s *RequiredSet) index(from, to VertexID) int {
	return int(from)*s.N + int(to)
}

// Add marks (from, to) as required.
func (s *RequiredSet) Add(from, to VertexID) {
	i := s.index(from, to)
	s.Bits[i/64] |= 1 << uint(i%64)
}

// Contains reports whether (from, to) is in the required set.
func (s *RequiredSet) Contains(from, to VertexID) bool {
	if s.N == 0 {
		return false
	}
	i := s.index(from, to)
	return s.Bits[i/64]&(1<<uint(i%64)) != 0
}
