package distance

import (
	"quantum-matrix/sdk"
)

type CalcOpt struct {
	Priority int  //优先级
	Open     bool // 是否开启
	Debug    bool // 是否为debug
	Strategy int  // strategy 驾车选择策略
	Method   int  // mode 出行方式
}
type distanceFunc func(lng1, lat1, lng2, lat2 float32) float32

// OptionStrategy  返回该sdk支持的策略，并将其映射至私有配置
func (opt *CalcOpt) OptionStrategy(handle func(map[int]interface{}) map[int]interface{}) map[int]interface{} {
	strategyMap := map[int]interface{}{
		sdk.Default: distanceFunc(func(lng1, lat1, lng2, lat2 float32) float32 {
			return sdk.Distance[float32](lng1, lat1, lng2, lat2)
		}),
		sdk.ShortestDistance: distanceFunc(func(lng1, lat1, lng2, lat2 float32) float32 {
			return sdk.Distance[float32](lng1, lat1, lng2, lat2)
		}),
		sdk.AvoidCongestion: distanceFunc(func(lng1, lat1, lng2, lat2 float32) float32 {
			return sdk.Distance[float32](lng1, lat1, lng2, lat2) * 1.5
		}),
		sdk.UnWalkFastRoute: distanceFunc(func(lng1, lat1, lng2, lat2 float32) float32 {
			return sdk.Distance[float32](lng1, lat1, lng2, lat2) * 1.5
		}),
	}
	return handle(strategyMap)
}

// OptionMethod  返回该sdk支持的出行方式，并将其映射至私有方法
func (opt *CalcOpt) OptionMethod(handle func(map[int]interface{}) map[int]interface{}) map[int]interface{} {
	methodMap := map[int]interface{}{
		sdk.Car:     7,
		sdk.Truck:   7,
		sdk.Bicycle: 7,
	}
	return handle(methodMap)
}

// OptionPriority  返回该配置的优先级
func (opt *CalcOpt) OptionPriority() int {
	return opt.Priority
}

// OptionOpen  返回该配置的状态
func (opt *CalcOpt) OptionOpen() bool {
	return opt.Open
}

// Update  根据map更新配置
func (opt *CalcOpt) Update(m map[string]interface{}) {
	sdk.MapToStruct(m, opt, "")
}

// Map  将配置结构体转为map后返回
func (opt *CalcOpt) Map() map[string]interface{} {
	return sdk.StructToMap(opt, "")
}

// LogDebug  是否开启该配置的详细debug，将会降低性能
func (opt *CalcOpt) LogDebug() bool {
	return opt.Debug
}

// Balance  通过某种算法实现负载均衡
func (opt *CalcOpt) Balance() interface{} {
	return nil
}

var _ sdk.IOptions = &CalcOpt{}

// DefaultDistanceOpt  默认配置
var DefaultDistanceOpt *CalcOpt = &CalcOpt{
	Priority: 0,
	Open:     true,
	Debug:    true,
	Strategy: sdk.Default,
	Method:   sdk.Car,
}
