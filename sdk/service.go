// Package sdk
// @Author Euraxluo  10:42:00
package sdk

import (
	"fmt"
	"github.com/zeromicro/go-zero/core/logx"
)

func Route(logger logx.Logger, waypoints [][2]float32, selective SDK, tripMethod int, strategy int, speed int) *LocationRouteServiceSchema {
	routingResult := selective.Routing(tripMethod, strategy, speed, waypoints...)
	if routingResult == nil {
		selective = Factory("", 100000)
		return selective.Routing(tripMethod, strategy, speed, waypoints...)
	}
	logger.Infof("Route::waypoints:%d, sdk:%#v, tripMethod:%d, strategy:%d, speed:%d, routesStrategy:%s", len(waypoints), selective, tripMethod, strategy, speed, routingResult.Routes.Strategy)
	return routingResult
}

func Matrix(logger logx.Logger, waypoints [][2]float32, sdk SDK, tripMethod int, strategy int, speed int) map[int]map[int]*Steps {
	var (
		route [][2]float32
	)
	sortedEdges := pointsPermutationsSorted(waypoints)
	for i, edge := range sortedEdges {
		route = append(route, edge[0])
		if i == len(sortedEdges)-1 {
			route = append(route, edge[1])
		}
	}
	routes := sdk.Routing(tripMethod, strategy, speed, route...)
	routes.Waypoint = waypoints
	logger.Infof("Matrix::waypoints:%d, sdk:%#v, tripMethod:%d, strategy:%d, speed:%d, routesStrategy:%s", len(waypoints), sdk, tripMethod, strategy, speed, routes.Routes.Strategy)
	return routes.LocationRouteServiceSchemaToMatrix()
}

// CoordConvertToGCJ02  将points 和对应经纬度，转换为 GCJ02 坐标的waypoints
func CoordConvertToGCJ02(points [][]float32, coordinate string) [][2]float32 {
	var waypoints [][2]float32
	switch coordinate {
	case GCJ02:
		waypoints = CoordListCast[float32, float32](points, func(x float32, y float32) (float32, float32) {
			return x, y
		})
	case BD09LL:
		waypoints = CoordListCast[float32, float32](points, BD09toGCJ02[float32])
	case WGS84:
		waypoints = CoordListCast[float32, float32](points, WGS84toGCJ02[float32])
	default:
		waypoints = CoordListCast[float32, float32](points, func(x float32, y float32) (float32, float32) {
			return x, y
		})
		logx.Error("coordinate type '" + coordinate + "' not sup")
	}
	return waypoints
}

// distinctPoint  去重
func distinctPoint(points [][2]float32) [][2]float32 {
	var result [][2]float32
	mapping := map[string]byte{}
	for _, p := range points {
		l := len(mapping)
		mapping[fmt.Sprint(p)] = 0
		if len(mapping) != l { // 加入map后，map长度变化，则元素不重复
			result = append(result, p)
		}
	}
	return result
}

// pointsPermutationsSorted  分配收集算法排序
func pointsPermutationsSorted(points [][2]float32) [][][2]float32 {
	adjacencyList := make(map[[2]float32]map[[2]float32]bool)
	allEdges := make([][][2]float32, 0)
	for edge := range Permutations(distinctPoint(points), 2) {
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
				//	如果结果集合为空,则随便取一个数据，记得同时移除两个集合
				edge, allEdges = allEdges[0], allEdges[1:]
				delete(adjacencyList[edge[0]], edge[1])

				// 构造一个边联通已排序数据和edge
				startNode = edge[0]
				sortedEdge = append(sortedEdge, [][2]float32{sortedEdge[len(sortedEdge)-1][1], startNode})
			} else {
				// 获取排序边集合的最后一个元素的终点,作为起点
				startNode = sortedEdge[len(sortedEdge)-1][1]
				// 据此起点,从邻接表中获取并弹出end_node
				for k, v := range adjacencyList[startNode] {
					if v == true {
						endNode = k
						break
					}
				}
				edge = [][2]float32{startNode, endNode}
				//  同时删除adjacencyList 和 allEdges 中的数据
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
		} else if len(sortedEdge) == 0 {
			//如果结果集合为空,则随便取一个数据
			//同时从两个集合中移除一个边
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
