package logic

import (
	"context"
	"distance-matrix/sdk"
	"reflect"

	"distance-matrix/internal/svc"
	"distance-matrix/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MatrixLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMatrixLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MatrixLogic {
	return &MatrixLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MatrixLogic) Matrix(req *types.DistanceMatrix) (resp *types.DistanceMatrixResponse, err error) {
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
	matrixEdges := sdk.Matrix(l.Logger, sdk.CoordConvertToGCJ02(req.Points, req.Coordinate), factorySdk, req.TripMethod, req.Strategy, req.Speed)
	var edges []types.Edge
	for i, m := range matrixEdges {
		for j, v := range m {
			edges = append(edges, types.Edge{
				I:           i,
				J:           j,
				Origin:      v.Origin,
				Destination: v.Destination,
				Duration:    v.Duration,
				Distance:    v.Distance,
				Speed:       v.Speed,
				Polyline: sdk.CoordCastList[float32, float32](v.Polyline, func(lon, lat float32) (float32, float32) {
					return lon, lat
				}),
			})
		}
	}
	return &types.DistanceMatrixResponse{
		Code: sdk.Ok.Code(),
		Msg:  sdk.Ok.Message(),
		Data: edges,
	}, nil
}
