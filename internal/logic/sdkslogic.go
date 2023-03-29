package logic

import (
	"context"
	"distance-matrix/common"
	"distance-matrix/sdk"
	"github.com/the-go-tool/cast"
	"reflect"

	"distance-matrix/internal/svc"
	"distance-matrix/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SdksLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSdksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SdksLogic {
	return &SdksLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SdksLogic) Sdks() (resp *types.ServiceListResponse, err error) {
	resp = &types.ServiceListResponse{
		Code: sdk.Ok.Code(),
		Msg:  sdk.Ok.Message(),
		Data: make(map[string]types.Options),
	}

	for k, _ := range common.GlobalConfig.SDK {
		resp.Data[k] = types.Options{
			Open:     sdk.Produce(k, true).Option().OptionOpen(),
			Priority: sdk.Produce(k, true).Option().OptionPriority(),
			Strategy: sdk.Produce(k, true).Option().OptionStrategy(func(m map[int]interface{}) map[int]interface{} {
				res := make(map[int]interface{})
				for n, f := range m {
					if reflect.ValueOf(f).Kind() == reflect.Func {
						res[n] = reflect.ValueOf(f).String()
					} else {
						res[n] = cast.To[string](f)
					}
				}
				return res
			}),
			Method: sdk.Produce(k, true).Option().OptionMethod(func(m map[int]interface{}) map[int]interface{} {
				res := make(map[int]interface{})
				for n, f := range m {
					if reflect.ValueOf(f).Kind() == reflect.Func {
						res[n] = reflect.ValueOf(f).String()
					} else {
						res[n] = cast.To[string](f)
					}
				}
				return res
			}),
			Option: sdk.Produce(k, true).Option().Map(),
		}
	}
	return
}
