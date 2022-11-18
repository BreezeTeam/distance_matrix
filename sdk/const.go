// Package sdk
// @Author Euraxluo  16:57:00
package sdk

//  TripMethod 出行方式
const (
	Car     = iota // 小汽车
	Truck          //货车
	Bicycle        //骑行
)

//  Coordinate 坐标类型
const (
	GCJ02  = "gcj02"
	BD09LL = "bd09ll"
	WGS84  = "WGS84"
)

//Strategy   策略
const (
	Default          = iota //厂家默认策略
	ShortestDistance        // 最短距离
	AvoidCongestion         // 躲避拥堵
	UnWalkFastRoute         // 不走高速
)
