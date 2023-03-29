package logic

import (
	"context"
	sdk "distance-matrix/sdk"
	"reflect"

	"distance-matrix/internal/svc"
	"distance-matrix/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RouteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRouteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RouteLogic {
	return &RouteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RouteLogic) Route(req *types.WaypointsRoute) (resp *types.WaypointsRouteResponse, err error) {
	//req.Coordinate  经纬度的类型，这个不用管，在sdk 接口中会自动转换
	//req.Strategy 想要使用的策略
	//Default          = iota //厂家默认策略
	//ShortestDistance        // 最短距离
	//AvoidCongestion         // 躲避拥堵
	//UnWalkFastRoute         // 不走高速

	//req.Speed 默认为7m/s
	//req.TripMethod 出行方式
	//Car     = iota // 小汽车
	//Truck          //货车
	//Bicycle        //骑行
	//req.Sdk //打算使用的sdk，默认是使用 最高优先级的，用于获取sdk
	factorySdk := sdk.Factory(req.Sdk, len(req.Points)/50)
	l.Logger.Infov(req)
	l.Logger.Infof("factorySdk:%s", reflect.TypeOf(factorySdk))
	route := sdk.Route(l.Logger, sdk.CoordConvertToGCJ02(req.Points, req.Coordinate), factorySdk, req.TripMethod, req.Strategy, req.Speed)
	return &types.WaypointsRouteResponse{
		Code: route.Code,
		Msg:  route.Message,
		Data: route.Routes,
	}, nil
}
