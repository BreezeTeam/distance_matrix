// Package restapi_amap_com
// @Author Euraxluo  15:34:00
package restapi_amap_com

import (
	"quantum-matrix/sdk"
	"strings"
)

type AmapOpt struct {
	Priority int    //优先级
	Open     bool   // 是否开启
	Debug    bool   // 是否为debug
	BasePath string // basePath 高德地图host
	Key      string // key 用户唯一标识
	Batch    int    // 途径点个数 默认为16

	// strategy
	Strategy string // strategy 驾车选择策略
	Method   int    // mode 出行方式
	Output   string // output 返回数据格式类型 默认为json

	Timeout         int                // 链接超时时间
	KeepAlive       int                // 链接KeepAlive 时间
	MaxIdleConns    int                // 最大空闲链接数
	IdleConnTimeout int                // 空闲链接超时时间
	Refresh         bool               //是否刷新
	LoadMapping     sdk.SmoothWeighted //负载
}

// OptionStrategy  返回该sdk支持的策略，并将其映射至私有配置
//1，躲避拥堵：返回的结果考虑路况，尽量躲避拥堵而规划路径；对应导航SDK货导策略12；
//2，不走高速：返回的结果考虑路况，不走高速；对应导航SDK货导策略13；
//3，避免收费：返回的结果考虑路况，尽可能规划收费较低甚至免费的路径；对应导航SDK货导策略14；
//4，躲避拥堵+不走高速：返回的结果考虑路况，尽量躲避拥堵，并且不走高速；对应导航SDK货导策略15；
//5，避免收费+不走高速：返回的结果考虑路况，尽量不走高速，并且尽量规划收费较低甚至免费的路径结果；对应导航SDK货导策略16；
//6，躲避拥堵+避免收费：返回的结果考虑路况，尽量的躲避拥堵，并且规划收费较低甚至免费的路径结果；对应导航SDK货导策略17；
//7，躲避拥堵+避免收费+不走高速：返回的结果考虑路况，尽量躲避拥堵，规划收费较低甚至免费的路径结果，并且尽量不走高速路；对应导航SDK货导策略18；
//8，高速优先：返回的结果考虑路况，会优先选择高速路；对应导航SDK货导策略19；
//9，躲避拥堵+高速优先：返回的结果考虑路况，会优先考虑高速路，并且会考虑路况躲避拥堵；对应导航SDK货导策略20；
//10，无路况速度优先：基于历史的通行速度数据，不考虑当前路况的影响，返回速度优先的路；如果不需要路况干扰计算结果，推荐使用此策略；（导航SDK货导策略无对应，真实导航时均会考虑路况）
//11，默认策略：返回的结果会考虑路况，躲避拥堵，速度优先以及费用优先；500Km规划以内会返回多条结果，500Km以外会返回单条结果；考虑路况情况下的综合最优策略，推荐使用；对应导航SDK货导策略10；
//12，无路况+不走高速：基于历史的通行速度数据，不考虑当前路况的影响，且不走高速路线，返回速度优先的路。
func (opt *AmapOpt) OptionStrategy(handle func(map[int]interface{}) map[int]interface{}) map[int]interface{} {
	strategyMap := map[int]interface{}{
		sdk.Default:          "11",
		sdk.ShortestDistance: "12",
		sdk.AvoidCongestion:  "1",
		sdk.UnWalkFastRoute:  "2",
	}
	return handle(strategyMap)
}

type RouteDirectionFunc func(s *SDK, strategy string, speed int, waypoints ...[2]float32) *sdk.LocationRouteServiceSchema

// OptionMethod  返回该sdk支持的出行方式，并将其映射至私有方法
func (opt *AmapOpt) OptionMethod(handle func(map[int]interface{}) map[int]interface{}) map[int]interface{} {
	methodMap := map[int]interface{}{
		sdk.Car:     RouteDirectionFunc(DirectionDrivingV3),
		sdk.Truck:   RouteDirectionFunc(DirectionDrivingV3),
		sdk.Bicycle: RouteDirectionFunc(DirectionDrivingV3),
	}
	return handle(methodMap)
}

// OptionPriority  返回该配置的优先级
func (opt *AmapOpt) OptionPriority() int {
	return opt.Priority
}

// OptionOpen  返回该配置的状态
func (opt *AmapOpt) OptionOpen() bool {
	return opt.Open
}

// Update  根据map更新配置
func (opt *AmapOpt) Update(m map[string]interface{}) {
	sdk.MapToStruct(m, opt, "")
	opt.Refresh = true
}

// Map  将配置结构体转为map后返回
func (opt *AmapOpt) Map() map[string]interface{} {
	return sdk.StructToMap(opt, "")
}

// LogDebug  是否开启该配置的详细debug，将会降低性能
func (opt *AmapOpt) LogDebug() bool {
	return opt.Debug
}

// Balance  通过某种算法实现负载均衡
func (opt *AmapOpt) Balance() interface{} {
	if opt.Refresh == true {
		counter := make(map[string]int)
		for _, k := range strings.Split(opt.Key, ",") {
			if _, ok := counter[k]; ok {
				counter[k] = counter[k] + 1
			} else {
				counter[k] = 1
			}
		}
		opt.LoadMapping.RemoveAll()
		//更新map
		for k, v := range counter {
			opt.LoadMapping.Add(k, v)
		}
		// 关闭刷新flag
		opt.Refresh = false
	}
	//否则从map中获取数据
	return opt.LoadMapping.Next()
}

var _ sdk.IOptions = &AmapOpt{}
