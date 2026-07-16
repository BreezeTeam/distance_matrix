package planner

import (
	"fmt"

	"distance-matrix/internal/geo"
)

// ChainOptimizer orders directed edges into a waypoint walk.
type ChainOptimizer interface {
	Order(points [][2]float32) [][][2]float32
}

// GreedyChain orders directed edges into a walk (Python point_pairing_sorted).
type GreedyChain struct{}

func (GreedyChain) Order(points [][2]float32) [][][2]float32 {
	return pointsPermutationsSorted(points)
}

// pointsPermutationsSorted — port of sdk/service.go allocation-collect algorithm.
func pointsPermutationsSorted(points [][2]float32) [][][2]float32 {
	points = distinctPoint(points)
	adjacencyList := make(map[[2]float32]map[[2]float32]bool)
	allEdges := make([][][2]float32, 0)
	for edge := range geo.Permutations(points, 2) {
		if _, ok := adjacencyList[edge[0]]; !ok {
			adjacencyList[edge[0]] = make(map[[2]float32]bool)
		}
		adjacencyList[edge[0]][edge[1]] = true
		allEdges = append(allEdges, edge)
	}

	var (
		edge      [][2]float32
		startNode [2]float32
		endNode   [2]float32
	)
	sortedEdge := make([][][2]float32, 0)
	for len(allEdges) > 0 {
		if len(sortedEdge) > 0 {
			if _, ok := adjacencyList[sortedEdge[len(sortedEdge)-1][1]]; !ok {
				edge, allEdges = allEdges[0], allEdges[1:]
				delete(adjacencyList[edge[0]], edge[1])
				startNode = edge[0]
				sortedEdge = append(sortedEdge, [][2]float32{sortedEdge[len(sortedEdge)-1][1], startNode})
			} else {
				startNode = sortedEdge[len(sortedEdge)-1][1]
				for k, v := range adjacencyList[startNode] {
					if v {
						endNode = k
						break
					}
				}
				edge = [][2]float32{startNode, endNode}
				delete(adjacencyList[edge[0]], edge[1])
				var remove int
				for k, v := range allEdges {
					if edge[0] == v[0] && edge[1] == v[1] {
						remove = k
						break
					}
				}
				allEdges = append(allEdges[:remove], allEdges[remove+1:]...)
			}
		} else {
			edge, allEdges = allEdges[0], allEdges[1:]
			delete(adjacencyList[edge[0]], edge[1])
			startNode = edge[0]
		}
		if len(adjacencyList[startNode]) == 0 {
			delete(adjacencyList, startNode)
		}
		sortedEdge = append(sortedEdge, edge)
	}
	return sortedEdge
}

func distinctPoint(points [][2]float32) [][2]float32 {
	var result [][2]float32
	mapping := map[string]byte{}
	for _, p := range points {
		l := len(mapping)
		mapping[fmt.Sprint(p)] = 0
		if len(mapping) != l {
			result = append(result, p)
		}
	}
	return result
}

// ChainToWaypoints flattens ordered edges into a waypoint polyline.
func ChainToWaypoints(edges [][][2]float32) [][2]float32 {
	var route [][2]float32
	for i, edge := range edges {
		route = append(route, edge[0])
		if i == len(edges)-1 {
			route = append(route, edge[1])
		}
	}
	return route
}
