package distance

import (
	"github.com/zeromicro/go-zero/core/logx"
	"path"
	"quantum-matrix/sdk"
	"reflect"
)

type SDK struct {
	opt *CalcOpt
}

func (s *SDK) Routing(method int, strategy int, speed int, waypoints ...[2]float32) *sdk.LocationRouteServiceSchema {
	//  此sdk OptionMethod 中表达的是 不同行驶方式的车速
	methodMapping := s.Option().OptionMethod(func(m map[int]interface{}) map[int]interface{} {
		return m
	})
	//  此sdk OptionMethod 能通过method获取速度
	methodSpeed, methodOk := methodMapping[method].(int)
	//  此sdk OptionStrategy 中表达的是 不用策略 对应的 不同的路径计算方式
	strategyMapping := s.Option().OptionStrategy(func(m map[int]interface{}) map[int]interface{} {
		return m
	})
	distance, distanceOk := strategyMapping[strategy].(distanceFunc)
	if !distanceOk {
		// 如果没有拿到distance函数，那么就获取默认的
		distance = strategyMapping[sdk.Default].(distanceFunc)
	}
	if !methodOk {
		// 如果没有获取到速度，那么默认为7
		methodSpeed = 7
	}
	speed = sdk.Min[int](methodSpeed, sdk.Max(speed, 7))
	return sdk.DefaultLocationRouteServiceSchema(sdk.Ok, float32(speed), distance, waypoints...)
}

func (s *SDK) Option() sdk.IOptions {
	return s.opt
}

var _ sdk.SDK = &SDK{}

// NewSDK  创建一个sdk服务，初始化http客户端和配置项
func NewSDK(opt sdk.IOptions) sdk.SDK {
	s := &SDK{}
	//  使用断言将interface 转为 struct
	s.opt = opt.(*CalcOpt)
	return s
}

// init  将该sdk注册到 sdk.FactoryByName 中
func init() {
	pkgName := path.Base(reflect.TypeOf(SDK{}).PkgPath())
	sdk.Register(pkgName, func(opt sdk.IOptions) sdk.SDK {
		return NewSDK(opt)
	})
	//  这里不用使用初始化的Opt，而是直接使用默认的Opt，同时也要将Update进行修改
	sdk.OptionRegister(pkgName, DefaultDistanceOpt)
	logx.Infof("pkg %s register as SDK success", pkgName)
}
