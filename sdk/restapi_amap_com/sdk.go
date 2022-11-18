// Package restapi_amap_com
// @Description: https://lbs.amap.com/api/webservice/guide/api/newroute#t5
package restapi_amap_com

import (
	"github.com/zeromicro/go-zero/core/logx"
	"net"
	"net/http"
	"path"
	"quantum-matrix/sdk"
	"reflect"
	"time"
)

type SDK struct {
	opt        *AmapOpt
	httpClient *http.Client
}

// Routing  在sdk内部，应当在此方法内较为通用的调用不通的策略和方法
func (s *SDK) Routing(method int, strategy int, speed int, waypoints ...[2]float32) *sdk.LocationRouteServiceSchema {
	//	获取 路径方法 mapping
	methodMapping := s.Option().OptionMethod(func(m map[int]interface{}) map[int]interface{} {
		return m
	})
	//	获取 路由函数
	routeDirectionFunc, MethodOk := methodMapping[method].(RouteDirectionFunc)
	//	获取高德策略
	amapStrategy, strategyOk := s.Option().OptionStrategy(func(m map[int]interface{}) map[int]interface{} {
		return m
	})[strategy].(string)
	if MethodOk && strategyOk {
		return routeDirectionFunc(s, amapStrategy, speed, waypoints...)
	} else {
		return nil
	}
}

func (s *SDK) Option() sdk.IOptions {
	return s.opt
}

var _ sdk.SDK = &SDK{}

// NewSDK  创建一个sdk服务，初始化http客户端和配置项
func NewSDK(opt sdk.IOptions) sdk.SDK {
	s := &SDK{}
	//  使用断言将interface 转为 struct
	s.opt = opt.(*AmapOpt)

	// 使用opt创建连接客户端链接
	s.httpClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   time.Duration(s.opt.Timeout) * time.Second,
				KeepAlive: time.Duration(s.opt.KeepAlive) * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          s.opt.MaxIdleConns,
			IdleConnTimeout:       time.Duration(s.opt.IdleConnTimeout) * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	return s
}

// init  将该sdk注册到 sdk.FactoryByName 中
func init() {
	pkgName := path.Base(reflect.TypeOf(SDK{}).PkgPath())
	sdk.Register(pkgName, func(opt sdk.IOptions) sdk.SDK {
		return NewSDK(opt)
	})
	sdk.OptionRegister(pkgName, new(AmapOpt))
	logx.Infof("pkg %s register as SDK success", pkgName)
}
