package arccover

// Scenario helpers for tests and benchmarks.

// CompleteRequired returns all directed pairs u→v, u≠v.
func CompleteRequired(n int) []Arc {
	out := make([]Arc, 0, n*(n-1))
	id := EdgeID(0)
	for u := 0; u < n; u++ {
		for v := 0; v < n; v++ {
			if u == v {
				continue
			}
			out = append(out, Arc{ID: id, From: VertexID(u), To: VertexID(v)})
			id++
		}
	}
	return out
}

// RandomMissRequired samples approximately density*N*(N-1) edges with fixed seed.
func RandomMissRequired(n int, density float64, seed uint64) []Arc {
	if density >= 1 {
		return CompleteRequired(n)
	}
	if density <= 0 {
		return nil
	}
	total := n * (n - 1)
	target := int(float64(total)*density + 0.5)
	if target < 1 {
		target = 1
	}
	if target > total {
		target = total
	}
	// Deterministic Fisher-Yates over edge index space without allocating all first when sparse.
	// For simplicity allocate all indices then take prefix after shuffle.
	idxs := make([]int, total)
	for i := range idxs {
		idxs[i] = i
	}
	state := seed
	for i := total - 1; i > 0; i-- {
		state = splitmix64(state)
		j := int(state % uint64(i+1))
		idxs[i], idxs[j] = idxs[j], idxs[i]
	}
	out := make([]Arc, 0, target)
	for t := 0; t < target; t++ {
		idx := idxs[t]
		// idx maps to (u,v) skipping self loops: for u in 0..n-1, v runs over n-1 targets.
		u := idx / (n - 1)
		off := idx % (n - 1)
		v := off
		if v >= u {
			v++
		}
		out = append(out, Arc{ID: EdgeID(t), From: VertexID(u), To: VertexID(v)})
	}
	return out
}

// RowMissRequired marks a fraction of origin rows as fully missing.
func RowMissRequired(n int, rowDensity float64, seed uint64) []Arc {
	if rowDensity <= 0 {
		return nil
	}
	rows := make([]int, n)
	for i := range rows {
		rows[i] = i
	}
	state := seed
	for i := n - 1; i > 0; i-- {
		state = splitmix64(state)
		j := int(state % uint64(i+1))
		rows[i], rows[j] = rows[j], rows[i]
	}
	k := int(float64(n)*rowDensity + 0.5)
	if k < 1 {
		k = 1
	}
	if k > n {
		k = n
	}
	out := make([]Arc, 0, k*(n-1))
	id := EdgeID(0)
	for i := 0; i < k; i++ {
		u := rows[i]
		for v := 0; v < n; v++ {
			if v == u {
				continue
			}
			out = append(out, Arc{ID: id, From: VertexID(u), To: VertexID(v)})
			id++
		}
	}
	return out
}

// OutStarRequired: edges from center 0 to all others.
func OutStarRequired(n int) []Arc {
	out := make([]Arc, 0, n-1)
	for v := 1; v < n; v++ {
		out = append(out, Arc{ID: EdgeID(v - 1), From: 0, To: VertexID(v)})
	}
	return out
}

// InStarRequired: edges from all others to center 0.
func InStarRequired(n int) []Arc {
	out := make([]Arc, 0, n-1)
	for v := 1; v < n; v++ {
		out = append(out, Arc{ID: EdgeID(v - 1), From: VertexID(v), To: 0})
	}
	return out
}

// PathRequired: 0→1→...→n-1
func PathRequired(n int) []Arc {
	if n < 2 {
		return nil
	}
	out := make([]Arc, 0, n-1)
	for i := 0; i < n-1; i++ {
		out = append(out, Arc{ID: EdgeID(i), From: VertexID(i), To: VertexID(i + 1)})
	}
	return out
}

// CycleRequired: 0→1→...→n-1→0
func CycleRequired(n int) []Arc {
	if n < 2 {
		return nil
	}
	out := PathRequired(n)
	out = append(out, Arc{ID: EdgeID(n - 1), From: VertexID(n - 1), To: 0})
	return out
}

// CycleWithChordRequired adds one chord 0→n/2.
func CycleWithChordRequired(n int) []Arc {
	out := CycleRequired(n)
	if n >= 4 {
		mid := n / 2
		out = append(out, Arc{ID: EdgeID(len(out)), From: 0, To: VertexID(mid)})
	}
	return out
}

// MultiComponentPaths builds k disjoint paths of length pathLen.
func MultiComponentPaths(k, pathLen int) (n int, required []Arc) {
	n = k * (pathLen + 1)
	id := EdgeID(0)
	for c := 0; c < k; c++ {
		base := c * (pathLen + 1)
		for i := 0; i < pathLen; i++ {
			required = append(required, Arc{
				ID: id, From: VertexID(base + i), To: VertexID(base + i + 1),
			})
			id++
		}
	}
	return n, required
}
