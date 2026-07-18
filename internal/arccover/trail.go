package arccover

// WeakComponents returns weakly connected component IDs for vertices that
// touch at least one required edge. Untouched vertices get -1.
func WeakComponents(n int, required []Arc) []int {
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}
	touched := make([]bool, n)
	for _, e := range required {
		touched[e.From] = true
		touched[e.To] = true
		union(int(e.From), int(e.To))
	}
	comp := make([]int, n)
	for i := 0; i < n; i++ {
		if !touched[i] {
			comp[i] = -1
			continue
		}
		comp[i] = find(i)
	}
	return comp
}

// MinimumTrailCount returns τ(G): minimum number of directed trails covering
// all required edges (polynomial via degree imbalance per weak component).
func MinimumTrailCount(n int, required []Arc) int {
	if len(required) == 0 {
		return 0
	}
	outDeg := make([]int, n)
	inDeg := make([]int, n)
	for _, e := range required {
		outDeg[e.From]++
		inDeg[e.To]++
	}
	comp := WeakComponents(n, required)

	type agg struct{ surplus int }
	groups := map[int]*agg{}
	for _, e := range required {
		c := comp[e.From]
		if groups[c] == nil {
			groups[c] = &agg{}
		}
	}
	for v := 0; v < n; v++ {
		c := comp[v]
		if c < 0 {
			continue
		}
		a := groups[c]
		if a == nil {
			continue
		}
		bal := outDeg[v] - inDeg[v]
		if bal > 0 {
			a.surplus += bal
		}
	}
	roots := make([]int, 0, len(groups))
	for r := range groups {
		roots = append(roots, r)
	}
	sortInts(roots)
	tau := 0
	for _, r := range roots {
		t := groups[r].surplus
		if t < 1 {
			t = 1
		}
		tau += t
	}
	return tau
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		j := i
		for j > 0 && a[j] < a[j-1] {
			a[j], a[j-1] = a[j-1], a[j]
			j--
		}
	}
}
