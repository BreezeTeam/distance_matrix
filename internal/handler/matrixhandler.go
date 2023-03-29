package handler

import (
	"net/http"

	"distance-matrix/internal/logic"
	"distance-matrix/internal/svc"
	"distance-matrix/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func matrixHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DistanceMatrix
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Error(w, err)
			return
		}

		l := logic.NewMatrixLogic(r.Context(), svcCtx)
		resp, err := l.Matrix(&req)
		if err != nil {
			httpx.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
