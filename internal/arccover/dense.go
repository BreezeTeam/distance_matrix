package arccover

import "context"

// DenseRouteFirst grows required-only fragments up to maxLegs using a fixed
// candidate window for successor selection.
func DenseRouteFirst(ctx context.Context, graph *MutableGraph, maxLegs int, candidateWindow int) []Fragment {
	if maxLegs <= 0 {
		return nil
	}
	if candidateWindow <= 0 {
		candidateWindow = 8
	}
	fragments := make([]Fragment, 0)
	starts := NewIndexedVertexHeap(graph)
	fid := 0

	for graph.RemainingEdges() > 0 {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		start := starts.PopBestWithOutgoing()
		if start == InvalidVertex {
			break
		}
		fragment := Fragment{Edges: make([]Arc, 0, maxLegs), ID: fid}
		current := start
		for len(fragment.Edges) < maxLegs {
			next, edge, ok := graph.SelectDenseNext(current, candidateWindow)
			if !ok {
				break
			}
			graph.Remove(edge.ID)
			starts.Update(edge.From)
			starts.Update(edge.To)
			fragment.Edges = append(fragment.Edges, edge)
			current = next
		}
		if len(fragment.Edges) > 0 {
			fragments = append(fragments, fragment)
			fid++
		}
	}

	for _, edge := range graph.RemainingArcs() {
		fragments = append(fragments, Fragment{Edges: []Arc{edge}, ID: fid})
		fid++
		graph.Remove(edge.ID)
	}
	return fragments
}

// DensePlan runs DenseRouteFirst + PackFragments.
func DensePlan(ctx context.Context, n int, required []Arc, maxLegs int, window int) Plan {
	g := NewMutableGraph(n, required)
	frags := DenseRouteFirst(ctx, g, maxLegs, window)
	reqSet := NewRequiredSet(n, required)
	routes := PackFragments(frags, maxLegs, reqSet)
	return Plan{
		Routes: routes,
		Stats: PlanStats{
			RequiredEdges: len(required),
			ProviderCalls: len(routes),
			Branch:        "dense",
		},
	}
}
