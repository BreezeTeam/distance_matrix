// Package restapi_amap_com
// @Author Euraxluo  9:04:00
package restapi_amap_com

import (
	"github.com/the-go-tool/cast"
	"github.com/zeromicro/go-zero/core/logx"
	"net/http"
	"net/url"
	"path"
	"quantum-matrix/sdk"
	"reflect"
)

// DirectionDrivingV3  骑行
func DirectionDrivingV3(s *SDK, strategy string, speed int, waypoints ...[2]float32) *sdk.LocationRouteServiceSchema {
	var (
		ec sdk.Errors
	)
	//	base path
	path := s.opt.BasePath + "/v3/direction/driving"
	//  strategy
	if strategy == "" {
		strategy = s.opt.Strategy
	}
	//  分批
	packets := sdk.WaypointsPacket(s.opt.Batch, waypoints...)
	//	运行器
	routeRunner := sdk.NewRouteRunner(len(packets))
	//  启动routeRunner
	for i, x := range packets {
		//根据负载均衡算法,获取key
		key := s.opt.Balance().(string)
		//构造函数
		queryParams := url.Values{}
		queryParams.Add("origin", sdk.ParameterToString(x[0], ","))
		queryParams.Add("waypoints", sdk.FloatEEToPolyline(x[1:len(x)-1], ";"))
		queryParams.Add("destination", sdk.ParameterToString(x[len(x)-1], ","))
		queryParams.Add("strategy", strategy)
		queryParams.Add("output", s.opt.Output)
		queryParams.Add("Key", key)
		nx := make([][2]float32, 0, len(x))
		nx = append(nx, x...)
		routeRunner.Run(i, func() *sdk.LocationRouteServiceSchema {
			//  对url进行请求
			if resp, err := sdk.LogRequest(path, queryParams, nil, s.opt.Debug, s.httpClient.Do, http.MethodPost)(); err == nil && resp.StatusCode == 200 {
				var v Response
				//  先尝试解码
				if err = sdk.DecodeResponse(&v, resp); err != nil {
					ec = sdk.Cause(err)
				} else if v.Route.Paths == nil || v.Status != "1" {
					ec = sdk.Obtain(v.Infocode + "-" + v.Info)
				} else {
					return v.Transformer(s, float32(speed), nx...)
				}
			} else if resp != nil {
				//  如果状态码为别的
				ec = sdk.Define(resp.StatusCode, resp.Status).Reload(sdk.Cause(err).Error())
			} else {
				ec = sdk.Cause(err)
			}
			logx.Errorf("%s %#v", path, queryParams)
			//  默认速度为7m/s
			return sdk.DefaultLocationRouteServiceSchema(ec, float32(sdk.Max(speed, 7)), sdk.Distance[float32], packets[i]...)
		})
	}
	//  收集结果
	return sdk.MergeLocationRouteServiceSchema(routeRunner.Results())
}

type StepCitiesTmcs struct {
	Lcode    []interface{} `json:"lcode"`
	Distance string        `json:"distance"`
	Status   string        `json:"status"`
	Polyline string        `json:"polyline"`
}
type CityDistricts struct {
	Name   string `json:"name"`
	Adcode string `json:"adcode"`
}

type StepCities struct {
	Name      string          `json:"name"`
	Citycode  string          `json:"citycode"`
	Adcode    string          `json:"adcode"`
	Districts []CityDistricts `json:"districts"`
}

// RoutePathsSteps - 导航路段
type RoutePathsSteps struct {
	Instruction     string           `json:"instruction"`
	Orientation     string           `json:"orientation"`
	Distance        string           `json:"distance"`
	Tolls           string           `json:"tolls"`
	TollDistance    string           `json:"toll_distance"`
	TollRoad        interface{}      `json:"toll_road"`
	Duration        string           `json:"duration"`
	Polyline        string           `json:"polyline"`
	Action          interface{}      `json:"action"`
	AssistantAction interface{}      `json:"assistant_action"`
	Tmcs            []StepCitiesTmcs `json:"tmcs"`
	Cities          []StepCities     `json:"cities"`
	Road            string           `json:"road"`
}

// RoutePaths - 驾车换乘方案
type RoutePaths struct {

	// 行驶距离 单位：米
	Distance string `json:"distance"`

	// 预计行驶时间 单位：秒
	Duration string `json:"duration"`

	// 导航策略
	Strategy string `json:"strategy"`

	// 此导航方案道路收费 单位：元
	Tolls string `json:"tolls"`

	// 限行结果 0 代表限行已规避或未限行，即该路线没有限行路段 1 代表限行无法规避，即该线路有限行路段
	Restriction string `json:"restriction"`

	// 红绿灯个数
	TrafficLights string `json:"traffic_lights"`

	// 收费路段距离
	TollDistance string `json:"toll_distance"`

	Steps []RoutePathsSteps `json:"steps"`
}

// Route - 驾车路径规划信息列表
type Route struct {

	// 起点坐标 规则： lon，lat（经度，纬度）， “,”分割，如117.500244, 40.417801 经纬度小数点不超过6位
	Origin string `json:"origin"`

	// 终点坐标 规则： lon，lat（经度，纬度）， “,”分割，如117.500244, 40.417801 经纬度小数点不超过6位
	Destination string `json:"destination"`

	// 打车费用 单位：元，注意：extensions=all时才会返回
	TaxiCost string `json:"taxi_cost"`

	Paths []RoutePaths `json:"paths"`
}

type Response struct {
	// 本次API访问状态，如果成功返回1，如果失败返回0。
	Status string `json:"status"`
	// 访问状态值的说明，如果成功返回\"ok\"，失败返回错误原因，具体见[错误码说明](https://lbs.amap.com/api/webservice/guide/tools/info)。
	Info string `json:"info"`
	// 返回状态说明,10000代表正确,详情参阅info状态表
	Infocode string `json:"infocode"`
	// 路径规划方案总数
	Count string `json:"count"`
	Route Route  `json:"route"`
}

func (r Response) Transformer(s *SDK, speed float32, waypoint ...[2]float32) *sdk.LocationRouteServiceSchema {
	ec := sdk.Ok
	//  解码成功，并且结果正确
	var steps []sdk.Steps
	CheckPoint := 1 // 检查点初始值
	for _, k := range r.Route.Paths[0].Steps {
		curCheckPoint := 0
		//计算是否需要add/append,公式: oldCheckPoint * (oldCheckPoint or curCheckPoint) = oldCheckPoint
		if val, ok := k.AssistantAction.(string); ok && sdk.In(val, []string{"到达途经地", "到达目的地"}) {
			curCheckPoint = 1
		}
		if CheckPoint*(CheckPoint|curCheckPoint) == 0 {
			lastStep := steps[len(steps)-1]
			lastStep.Distance += cast.To[float32](k.Distance)
			lastStep.Duration += cast.To[float32](k.Duration)
			//lastStep.Polyline = append(lastStep.Polyline, sdk.PolylineToFloatEE(k.Polyline, ";")...)
			lastStep.Speed = lastStep.Distance / lastStep.Duration
			steps[len(steps)-1] = lastStep
		} else {
			steps = append(steps, sdk.Steps{
				Origin:      waypoint[len(steps)],
				Destination: waypoint[len(steps)+1],
				Distance:    cast.To[float32](k.Distance),
				Duration:    cast.To[float32](k.Duration),
				//Polyline:    sdk.PolylineToFloatEE(k.Polyline, ";"),
				Polyline: [][2]float32{waypoint[len(steps)], waypoint[len(steps)+1]},
				Speed:    cast.To[float32](k.Distance) / cast.To[float32](k.Duration),
			})
		}
		CheckPoint = curCheckPoint
	}

	return &sdk.LocationRouteServiceSchema{Code: ec.Code(),
		Message:  ec.Message(),
		Waypoint: waypoint,
		Routes: sdk.Routes{
			Distance: cast.To[float32](r.Route.Paths[0].Distance),
			Duration: cast.To[float32](r.Route.Paths[0].Duration),
			Tolls:    cast.To[float32](r.Route.Paths[0].Tolls),
			Strategy: path.Base(reflect.TypeOf(SDK{}).PkgPath()) + "-" + s.opt.BasePath + "-" + cast.To[string](r.Route.Paths[0].Strategy),
			Steps:    steps,
		},
	}
}
