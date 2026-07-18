package arccover

// MutableGraph stores remaining required arcs with adjacency lists.
type MutableGraph struct {
	N         int
	Out       [][]EdgeID
	Edges     []Arc
	Alive     []bool
	OutDegree []int
	InDegree  []int
	Remaining int
}

// NewMutableGraph builds a graph from required arcs (IDs must be 0..m-1).
func NewMutableGraph(n int, required []Arc) *MutableGraph {
	g := &MutableGraph{
		N:         n,
		Out:       make([][]EdgeID, n),
		Edges:     make([]Arc, len(required)),
		Alive:     make([]bool, len(required)),
		OutDegree: make([]int, n),
		InDegree:  make([]int, n),
		Remaining: len(required),
	}
	copy(g.Edges, required)
	for i, e := range required {
		g.Alive[i] = true
		g.Out[e.From] = append(g.Out[e.From], e.ID)
		g.OutDegree[e.From]++
		g.InDegree[e.To]++
	}
	// Sort each adjacency by To, then EdgeID for determinism.
	for u := 0; u < n; u++ {
		sortEdgeIDs(g.Out[u], g.Edges)
	}
	return g
}

func sortEdgeIDs(ids []EdgeID, edges []Arc) {
	for i := 1; i < len(ids); i++ {
		j := i
		for j > 0 {
			a, b := edges[ids[j-1]], edges[ids[j]]
			if a.To < b.To || (a.To == b.To && a.ID <= b.ID) {
				break
			}
			ids[j-1], ids[j] = ids[j], ids[j-1]
			j--
		}
	}
}

// RemainingEdges returns how many alive required edges remain.
func (g *MutableGraph) RemainingEdges() int {
	return g.Remaining
}

// Remove marks an edge dead and updates degrees. Uses swap-remove from Out list.
func (g *MutableGraph) Remove(id EdgeID) {
	if !g.Alive[id] {
		return
	}
	g.Alive[id] = false
	g.Remaining--
	e := g.Edges[id]
	g.OutDegree[e.From]--
	g.InDegree[e.To]--

	// Swap-remove from Out[from] for O(1) amortized live scans.
	out := g.Out[e.From]
	for i, eid := range out {
		if eid == id {
			last := len(out) - 1
			out[i] = out[last]
			g.Out[e.From] = out[:last]
			// Keep remaining suffix sorted? Dense only needs a small window;
			// re-sort is expensive. Instead leave unsorted and SelectDenseNext
			// scans live list which is now compact.
			return
		}
	}
}

// Clone returns a deep copy.
func (g *MutableGraph) Clone() *MutableGraph {
	cp := &MutableGraph{
		N:         g.N,
		Out:       make([][]EdgeID, g.N),
		Edges:     make([]Arc, len(g.Edges)),
		Alive:     make([]bool, len(g.Alive)),
		OutDegree: make([]int, g.N),
		InDegree:  make([]int, g.N),
		Remaining: g.Remaining,
	}
	copy(cp.Edges, g.Edges)
	copy(cp.Alive, g.Alive)
	copy(cp.OutDegree, g.OutDegree)
	copy(cp.InDegree, g.InDegree)
	for u := 0; u < g.N; u++ {
		cp.Out[u] = append([]EdgeID(nil), g.Out[u]...)
	}
	return cp
}

// RemainingArcs returns alive arcs in ascending EdgeID order.
func (g *MutableGraph) RemainingArcs() []Arc {
	out := make([]Arc, 0, g.Remaining)
	for i, alive := range g.Alive {
		if alive {
			out = append(out, g.Edges[i])
		}
	}
	return out
}

// SelectDenseNext picks the next vertex among up to window live out-neighbors
// maximizing (out-in, out, -v). Out lists are kept compact by Remove.
func (g *MutableGraph) SelectDenseNext(u VertexID, window int) (VertexID, Arc, bool) {
	if window <= 0 {
		window = 8
	}
	bestOK := false
	var bestV VertexID
	var bestE Arc
	bestBal := 0
	bestOut := 0
	checked := 0
	for _, eid := range g.Out[u] {
		e := g.Edges[eid]
		v := e.To
		bal := g.OutDegree[v] - g.InDegree[v]
		out := g.OutDegree[v]
		if !bestOK || bal > bestBal || (bal == bestBal && out > bestOut) ||
			(bal == bestBal && out == bestOut && v < bestV) {
			bestOK = true
			bestV = v
			bestE = e
			bestBal = bal
			bestOut = out
		}
		checked++
		if checked >= window {
			break
		}
	}
	return bestV, bestE, bestOK
}

// FirstAliveOut returns the first alive out-edge of u, if any.
func (g *MutableGraph) FirstAliveOut(u VertexID) (Arc, bool) {
	for _, eid := range g.Out[u] {
		if g.Alive[eid] {
			return g.Edges[eid], true
		}
	}
	return Arc{}, false
}

// IndexedVertexHeap picks start vertices by (out-in, out, -v).
type IndexedVertexHeap struct {
	g      *MutableGraph
	order  []VertexID
	pos    []int
	active []bool
}

// NewIndexedVertexHeap builds a heap over all vertices with outgoing edges.
func NewIndexedVertexHeap(g *MutableGraph) *IndexedVertexHeap {
	h := &IndexedVertexHeap{
		g:      g,
		order:  make([]VertexID, 0, g.N),
		pos:    make([]int, g.N),
		active: make([]bool, g.N),
	}
	for i := range h.pos {
		h.pos[i] = -1
	}
	for v := 0; v < g.N; v++ {
		if g.OutDegree[v] > 0 {
			h.push(VertexID(v))
		}
	}
	return h
}

func (h *IndexedVertexHeap) better(a, b VertexID) bool {
	ga, gb := h.g, h.g
	balA := ga.OutDegree[a] - ga.InDegree[a]
	balB := gb.OutDegree[b] - gb.InDegree[b]
	if balA != balB {
		return balA > balB
	}
	if ga.OutDegree[a] != gb.OutDegree[b] {
		return ga.OutDegree[a] > gb.OutDegree[b]
	}
	return a < b
}

func (h *IndexedVertexHeap) push(v VertexID) {
	if h.active[v] {
		return
	}
	h.active[v] = true
	h.pos[v] = len(h.order)
	h.order = append(h.order, v)
	h.siftUp(h.pos[v])
}

func (h *IndexedVertexHeap) Update(v VertexID) {
	if h.g.OutDegree[v] <= 0 {
		h.remove(v)
		return
	}
	if !h.active[v] {
		h.push(v)
		return
	}
	i := h.pos[v]
	h.siftUp(i)
	h.siftDown(h.pos[v])
}

func (h *IndexedVertexHeap) remove(v VertexID) {
	if !h.active[v] {
		return
	}
	i := h.pos[v]
	last := len(h.order) - 1
	h.swap(i, last)
	h.order = h.order[:last]
	h.active[v] = false
	h.pos[v] = -1
	if i < len(h.order) {
		h.siftUp(i)
		h.siftDown(i)
	}
}

func (h *IndexedVertexHeap) swap(i, j int) {
	h.order[i], h.order[j] = h.order[j], h.order[i]
	h.pos[h.order[i]] = i
	h.pos[h.order[j]] = j
}

func (h *IndexedVertexHeap) siftUp(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if !h.better(h.order[i], h.order[p]) {
			break
		}
		h.swap(i, p)
		i = p
	}
}

func (h *IndexedVertexHeap) siftDown(i int) {
	n := len(h.order)
	for {
		l := 2*i + 1
		if l >= n {
			break
		}
		best := l
		r := l + 1
		if r < n && h.better(h.order[r], h.order[l]) {
			best = r
		}
		if !h.better(h.order[best], h.order[i]) {
			break
		}
		h.swap(i, best)
		i = best
	}
}

// PopBestWithOutgoing pops the best vertex that still has outgoing edges.
func (h *IndexedVertexHeap) PopBestWithOutgoing() VertexID {
	for len(h.order) > 0 {
		v := h.order[0]
		h.remove(v)
		if h.g.OutDegree[v] > 0 {
			return v
		}
	}
	return InvalidVertex
}
