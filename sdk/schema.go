// Package sdk
// @Author Euraxluo  15:30:00
package sdk

import (
	"fmt"
	"strconv"
)

type LocationRouteServiceSchema struct {
	// Code  状态码
	Code int `json:"code"`
	// Message  状态码对应的信息
	Message string `json:"message"`
	// Waypoint  途径点，包括起点和终点
	Waypoint [][2]float32 `json:"waypoint"`
	// Routes  状态码
	Routes Routes `json:"routes"`
}

type Routes struct {
	// Distance  方案距离
	Distance float32 `json:"distance"`
	// Duration  方案时间
	Duration float32 `json:"duration"`
	// Tolls  此导航方案道路收费 单位：元
	Tolls float32 `json:"tolls"`
	// Strategy  策略，这里会给出sdk名字+接口+策略
	Strategy string `json:"strategy"`
	// Steps 路线分段，此处每一个路线就是 Waypoint 中 点对 间的分段
	Steps []Steps `json:"steps"`
}

type Steps struct {
	// origin  起点
	Origin [2]float32 `json:"origin"`
	// Destination  终点
	Destination [2]float32 `json:"destination"`
	// Duration  该分段的时间
	Duration float32 `json:"duration"`
	// Distance  该分段的距离
	Distance float32 `json:"distance"`
	// Polyline  该分段点坐标序列
	Polyline [][2]float32 `json:"polyline"`
	// Speed  该分段的平均速度
	Speed float32 `json:"speed"`
}

func (l *LocationRouteServiceSchema) LocationRouteServiceSchemaToMatrix() map[int]map[int]*Steps {
	edgeMapping := make(map[int]map[int]*Steps)
	stepMapping := make(map[[2]float32]map[[2]float32]*Steps)
	for i := 0; i < len(l.Routes.Steps); i++ {
		step := l.Routes.Steps[i]
		if _, ok := stepMapping[step.Origin]; !ok {
			stepMapping[step.Origin] = make(map[[2]float32]*Steps)
		}
		stepMapping[step.Origin][step.Destination] = &step
	}

	for oId, i := range l.Waypoint {
		for dId, j := range l.Waypoint {
			if _, ok := edgeMapping[oId]; !ok {
				edgeMapping[oId] = make(map[int]*Steps)
			}
			//  如果起点等于终点，那么，设为0
			if oId == dId || i == j {
				edgeMapping[oId][dId] = &Steps{
					Origin:      l.Waypoint[oId],
					Destination: l.Waypoint[dId],
					Duration:    0,
					Distance:    0,
					Polyline:    [][2]float32{i, j},
					Speed:       0,
				}
				continue
			}
			if val, ok := stepMapping[i][j]; ok {
				edgeMapping[oId][dId] = val
			} else {
				fmt.Printf("%v 不存在的边 \n", [][2]float32{i, j})
				distance := Distance(i[0], i[1], j[0], j[1])
				edgeMapping[oId][dId] = &Steps{
					Origin:      l.Waypoint[oId],
					Destination: l.Waypoint[dId],
					Distance:    distance,
					Duration:    distance / 7,
					Polyline:    [][2]float32{i, j},
					Speed:       7,
				}
			}

		}
	}
	return edgeMapping
}

func DefaultLocationRouteServiceSchema(ec Errors, speed float32, distanceFunc func(lng1, lat1, lng2, lat2 float32) float32, waypoints ...[2]float32) *LocationRouteServiceSchema {
	var (
		steps         []Steps
		totalDistance float32
		totalDuration float32
	)
	if len(waypoints) > 1 {
		for _, pair := range Zip(waypoints, waypoints[1:]) {
			distance := distanceFunc(pair.First[0], pair.First[1], pair.Second[0], pair.Second[1])
			totalDistance += distance
			totalDuration += distance / speed
			steps = append(steps, Steps{
				Origin:      pair.First,
				Destination: pair.Second,
				Distance:    distance,
				Duration:    distance / speed, // 7m/s == 25.2km/h
				Polyline:    [][2]float32{pair.First, pair.Second},
				Speed:       speed,
			})
		}
	}
	if ec == nil {
		ec = Ok
	}
	return &LocationRouteServiceSchema{
		Code:     ec.Code(),
		Message:  ec.Message(),
		Waypoint: waypoints,
		Routes: Routes{
			Distance: totalDistance,
			Duration: totalDuration,
			Tolls:    0,
			Strategy: "Default",
			Steps:    steps,
		},
	}
}

func MergeLocationRouteServiceSchema(routes []*LocationRouteServiceSchema) *LocationRouteServiceSchema {
	var (
		waypoints     [][2]float32
		totalDistance float32
		totalDuration float32
		totalTolls    float32
		strategy      string
		steps         []Steps
	)
	strategyMapping := make(map[string]int)
	for _, k := range routes {
		if len(waypoints) == 0 {
			waypoints = append(waypoints, k.Waypoint...)
		} else {
			waypoints = append(waypoints, k.Waypoint[1:]...)
		}
		totalDistance += k.Routes.Distance
		totalDuration += k.Routes.Duration
		totalTolls += k.Routes.Tolls
		if _, ok := strategyMapping[k.Routes.Strategy]; ok {
			strategyMapping[k.Routes.Strategy] += 1
		} else {
			strategyMapping[k.Routes.Strategy] = 1
		}
		steps = append(steps, k.Routes.Steps...)
	}
	strategy = ""
	for k, v := range strategyMapping {
		strategy += k + "x" + strconv.Itoa(v) + ";"
	}
	return &LocationRouteServiceSchema{
		Code:     Ok.Code(),
		Message:  Ok.Message(),
		Waypoint: waypoints,
		Routes: Routes{
			Distance: totalDistance,
			Duration: totalDuration,
			Tolls:    totalTolls,
			Strategy: strategy,
			Steps:    steps,
		},
	}
}
