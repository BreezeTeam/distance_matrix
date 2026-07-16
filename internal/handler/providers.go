package handler

import (
	"net/http"

	"distance-matrix/internal/svc"
	"distance-matrix/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func providersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.OkJson(w, types.ProvidersResponse{
			Code: 200,
			Msg:  "OK",
			Data: svcCtx.Registry.List(),
		})
	}
}
