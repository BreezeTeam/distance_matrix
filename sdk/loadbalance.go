// Package sdk
// @Author Euraxluo  15:40:00
package sdk

type smoothWeighted struct {
	Item            interface{}
	Weight          int
	CurrentWeight   int
	EffectiveWeight int
}

type SmoothWeighted struct {
	items []*smoothWeighted
	n     int
}

// Add a weighted server.
func (w *SmoothWeighted) Add(item interface{}, weight int) {
	weighted := &smoothWeighted{Item: item, Weight: weight, EffectiveWeight: weight}
	w.items = append(w.items, weighted)
	w.n++
}

// RemoveAll removes all weighted items.
func (w *SmoothWeighted) RemoveAll() {
	w.items = w.items[:0]
	w.n = 0
}

//Reset resets all current weights.
func (w *SmoothWeighted) Reset() {
	for _, s := range w.items {
		s.EffectiveWeight = s.Weight
		s.CurrentWeight = 0
	}
}

// All returns all items.
func (w *SmoothWeighted) All() map[interface{}]int {
	m := make(map[interface{}]int)
	for _, i := range w.items {
		m[i.Item] = i.Weight
	}
	return m
}

// Next returns next selected server.
func (w *SmoothWeighted) Next() interface{} {
	i := w.nextWeighted()
	if i == nil {
		return nil
	}
	return i.Item
}

// nextWeighted returns next selected weighted object.
func (w *SmoothWeighted) nextWeighted() *smoothWeighted {
	if w.n == 0 {
		return nil
	}
	if w.n == 1 {
		return w.items[0]
	}

	return nextSmoothWeighted(w.items)
}

//https://github.com/phusion/nginx/commit/27e94984486058d73157038f7950a0a36ecc6e35
func nextSmoothWeighted(items []*smoothWeighted) (best *smoothWeighted) {
	total := 0

	for i := 0; i < len(items); i++ {
		w := items[i]

		if w == nil {
			continue
		}

		w.CurrentWeight += w.EffectiveWeight
		total += w.EffectiveWeight
		if w.EffectiveWeight < w.Weight {
			w.EffectiveWeight++
		}

		if best == nil || w.CurrentWeight > best.CurrentWeight {
			best = w
		}

	}

	if best == nil {
		return nil
	}

	best.CurrentWeight -= total
	return best
}
