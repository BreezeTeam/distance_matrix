package arccover

// PackFragments packs required-only fragments into routes of capacity maxLegs,
// using bucketed best-fit with size s_i = len+1 into bins of capacity L+1.
//
// Selection rule (unchanged): exact size first, else largest fit; among a size,
// prefer a non-duplicate bridge (end→start ∉ required), then minimum fragment ID.
//
// Hot path (row/star graphs with tens of thousands of tiny fragments) indexes
// each size bucket by fragment Start() so pickFit is O(#starts) instead of
// O(#fragments-in-bucket).
func PackFragments(fragments []Fragment, maxLegs int, requiredSet RequiredSet) []Route {
	if maxLegs <= 0 || len(fragments) == 0 {
		return nil
	}
	capacity := maxLegs + 1 // sizes are len+1
	n := requiredSet.N
	if n <= 0 {
		n = 1
		for _, f := range fragments {
			if f.Len() == 0 {
				continue
			}
			if int(f.Start())+1 > n {
				n = int(f.Start()) + 1
			}
			if int(f.End())+1 > n {
				n = int(f.End()) + 1
			}
		}
	}

	type item struct {
		f     Fragment
		size  int
		start VertexID
	}
	items := make([]item, len(fragments))
	// buckets[s] = item indices sorted by fragment ID (append order).
	buckets := make([][]int, capacity+1)
	bucketHead := make([]int, capacity+1)
	// byStart[s][v] = item indices with Start==v, ID-sorted; startHead cursors.
	byStart := make([][][]int, capacity+1)
	startHead := make([][]int, capacity+1)
	for s := 1; s <= capacity; s++ {
		byStart[s] = make([][]int, n)
		startHead[s] = make([]int, n)
	}
	// activeStarts[s] = starts that still have unread entries (for O(#starts) scans).
	activeStarts := make([][]VertexID, capacity+1)

	for i, f := range fragments {
		sz := f.Len() + 1
		if sz > capacity {
			sz = capacity
		}
		if sz < 1 {
			sz = 1
		}
		st := VertexID(0)
		if f.Len() > 0 {
			st = f.Start()
		}
		items[i] = item{f: f, size: sz, start: st}
		buckets[sz] = append(buckets[sz], i)
		if int(st) < n {
			if len(byStart[sz][st]) == 0 {
				activeStarts[sz] = append(activeStarts[sz], st)
			}
			byStart[sz][st] = append(byStart[sz][st], i)
		}
	}

	used := make([]bool, len(items))
	remaining := len(items)
	routes := make([]Route, 0, ceilDiv(remaining, 1))

	peekStart := func(s int, st VertexID) int {
		if int(st) >= n {
			return -1
		}
		h := startHead[s][st]
		list := byStart[s][st]
		for h < len(list) {
			idx := list[h]
			if !used[idx] {
				startHead[s][st] = h
				return idx
			}
			h++
		}
		startHead[s][st] = h
		return -1
	}

	// pickFromSize: first non-dup by min ID, else first any by min ID.
	pickFromSize := func(s int, end VertexID) int {
		best := -1
		bestID := int(^uint(0) >> 1)
		starts := activeStarts[s]
		w := 0
		for _, st := range starts {
			idx := peekStart(s, st)
			if idx < 0 {
				continue
			}
			starts[w] = st
			w++
			if requiredSet.Contains(end, st) {
				continue
			}
			id := items[idx].f.ID
			if best < 0 || id < bestID {
				best = idx
				bestID = id
			}
		}
		activeStarts[s] = starts[:w]
		if best >= 0 {
			return best
		}
		// All remaining are duplicate bridges (or empty non-dup set): min ID any.
		starts = activeStarts[s]
		w = 0
		best = -1
		bestID = int(^uint(0) >> 1)
		for _, st := range starts {
			idx := peekStart(s, st)
			if idx < 0 {
				continue
			}
			starts[w] = st
			w++
			id := items[idx].f.ID
			if best < 0 || id < bestID {
				best = idx
				bestID = id
			}
		}
		activeStarts[s] = starts[:w]
		return best
	}

	popBestSize := func(maxSize int) int {
		for s := maxSize; s >= 1; s-- {
			b := buckets[s]
			h := bucketHead[s]
			for h < len(b) {
				idx := b[h]
				h++
				if used[idx] {
					continue
				}
				bucketHead[s] = h
				return idx
			}
			bucketHead[s] = h
		}
		return -1
	}

	pickFit := func(space int, end VertexID) int {
		if space >= 1 && space <= capacity {
			if idx := pickFromSize(space, end); idx >= 0 {
				return idx
			}
		}
		for s := minInt(space, capacity); s >= 1; s-- {
			if s == space {
				continue
			}
			if idx := pickFromSize(s, end); idx >= 0 {
				return idx
			}
		}
		return -1
	}

	for remaining > 0 {
		startIdx := popBestSize(capacity)
		if startIdx < 0 {
			break
		}
		used[startIdx] = true
		remaining--

		cur := items[startIdx]
		route := fragmentToRoute(cur.f)
		space := capacity - cur.size
		end := cur.f.End()

		for space > 0 && remaining > 0 {
			best := pickFit(space, end)
			if best < 0 {
				break
			}
			used[best] = true
			remaining--
			next := items[best]
			space -= next.size
			route.Legs = append(route.Legs, Leg{
				From: end,
				To:   next.f.Start(),
				Kind: LegBridge,
			})
			route.Legs = append(route.Legs, fragmentLegs(next.f)...)
			end = next.f.End()
		}
		routes = append(routes, route)
	}
	return routes
}

func fragmentLegs(f Fragment) []Leg {
	legs := make([]Leg, len(f.Edges))
	for i, e := range f.Edges {
		legs[i] = Leg{From: e.From, To: e.To, Kind: LegRequired, EdgeID: e.ID}
	}
	return legs
}

func fragmentToRoute(f Fragment) Route {
	return Route{Legs: fragmentLegs(f)}
}

// SplitFragmentsByMaxLegs chops trails longer than maxLegs into pieces.
func SplitFragmentsByMaxLegs(fragments []Fragment, maxLegs int) []Fragment {
	if maxLegs <= 0 {
		return fragments
	}
	out := make([]Fragment, 0, len(fragments))
	id := 0
	for _, f := range fragments {
		if f.Len() <= maxLegs {
			f.ID = id
			id++
			out = append(out, f)
			continue
		}
		for start := 0; start < f.Len(); start += maxLegs {
			end := minInt(start+maxLegs, f.Len())
			out = append(out, Fragment{
				Edges: append([]Arc(nil), f.Edges[start:end]...),
				ID:    id,
			})
			id++
		}
	}
	return out
}

// RoutesToRequiredFragments extracts required-only fragments from routes.
func RoutesToRequiredFragments(routes []Route) []Fragment {
	var frags []Fragment
	id := 0
	for _, r := range routes {
		var cur []Arc
		flush := func() {
			if len(cur) == 0 {
				return
			}
			frags = append(frags, Fragment{Edges: append([]Arc(nil), cur...), ID: id})
			id++
			cur = cur[:0]
		}
		for _, leg := range r.Legs {
			if leg.Kind == LegBridge {
				flush()
				continue
			}
			cur = append(cur, Arc{ID: leg.EdgeID, From: leg.From, To: leg.To})
		}
		flush()
	}
	return frags
}
