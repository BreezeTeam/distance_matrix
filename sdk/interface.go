package sdk

import (
	"github.com/zeromicro/go-zero/core/logx"
)

// IOptions
// 1. sdk配置
// 2. 配置的状态存储
type IOptions interface {
	// Balance  状态均衡
	Balance() interface{}

	// LogDebug  返回是否debug
	LogDebug() bool

	// OptionOpen  返回是否开启配置
	OptionOpen() bool

	// OptionPriority  优先级
	OptionPriority() int

	// OptionStrategy  支持的策略，这里是 interface{} 的 原因是因为 可能返回的是不通的方法，但是也可能是一个负载的option
	// 如果 interface{} 是较为负载的结构，应当在 sdk 中 type 定义后使用
	OptionStrategy(func(map[int]interface{}) map[int]interface{}) map[int]interface{}

	// OptionMethod  支持的出行方式,这里结果是 interface{} 是因为，可能这里是存储的 不通的 车速，或是调用不通的链接，或者就是不通的方法
	// 如果 interface{} 是较为负载的结构，应当在 sdk 中 type 定义后使用
	OptionMethod(func(map[int]interface{}) map[int]interface{}) map[int]interface{}

	// Update  根据字典进行配置更新
	Update(map[string]interface{})

	// Map  将option转为字典类型
	Map() map[string]interface{}
}

// SDK  sdk接口定义
type SDK interface {
	// Routing  进行routing计算
	Routing(method int, strategy int, speed int, waypoints ...[2]float32) *LocationRouteServiceSchema
	// Option  获取sdk的opt
	Option() IOptions
}
type RoutingFunc func(method int, strategy int, speed int, waypoints ...[2]float32) *LocationRouteServiceSchema

func (f RoutingFunc) Routing(method int, strategy int, speed int, waypoints ...[2]float32) *LocationRouteServiceSchema {
	return f(method, strategy, speed, waypoints...)
}

var (
	FactoryByName       = make(map[string]func(opt IOptions) SDK)
	OptionFactoryByName = make(map[string]IOptions)
	InstanceByName      = make(map[string]SDK)
)

// Factory  返回一个可用的优先级最高的SDK
func Factory(name string, pointCounts int) SDK {
	var (
		result   SDK
		priority int
	)
	//  如果输入的sdk名字不为空，并且是打开的状态，那么就使用该sdk
	if name != "" {
		sdk := Produce(name, true)
		if sdk.Option().OptionOpen() && sdk.Option().OptionPriority() >= priority {
			result = sdk
			return result
		}
	}
	//  如果点数目>50,则使用最低优先级的
	if pointCounts > 50 {
		for k, _ := range FactoryByName {
			sdk := Produce(k, true)
			if sdk.Option().OptionOpen() && sdk.Option().OptionPriority() <= priority {
				result = sdk
				priority = sdk.Option().OptionPriority()
			}
		}
	} else {
		//  否则按照优先级返回，返回优先级最大的那个sdk
		for k, _ := range FactoryByName {
			sdk := Produce(k, true)
			if sdk.Option().OptionOpen() && sdk.Option().OptionPriority() >= priority {
				result = sdk
				priority = sdk.Option().OptionPriority()
			}
		}
	}
	return result
}

// Register  SDK注册函数，将SDK注册到工厂表中
// TODO: 实现加载优先级的设置
func Register(name string, factory func(opt IOptions) SDK) {
	FactoryByName[name] = factory
}

// OptionRegister  Option 注册函数，将 Option 注册到工厂表中
func OptionRegister(name string, options IOptions) {
	OptionFactoryByName[name] = options
}

// Produce 通过名字获取SDK的函数，single为true时，优先从实例表中查询
func Produce(name string, single bool) SDK {
	if opt, ok := OptionFactoryByName[name]; !ok {
		return nil
	} else {
		//  如果为单例，则先去实例map中寻找，找不着再去创建
		if instance, ok := InstanceByName[name]; ok {
			if single {
				return instance
			} else {
				return GetSdkWithName(name, opt)
			}
		} else if sdk := GetSdkWithName(name, opt); sdk != nil {
			InstanceByName[name] = sdk
			return sdk
		}
	}
	return nil
}

// GetSdkWithName  根据名字获取SDK，如果没有则返回nil
func GetSdkWithName(name string, opt IOptions) SDK {
	if sdk, ok := FactoryByName[name]; ok {
		return sdk(opt)
	}
	logx.Error("sdk name {" + name + "} not found,plz import first!")
	return nil
}

//TODO:实现按照优先级获取sdk
