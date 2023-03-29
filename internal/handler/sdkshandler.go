package handler

import (
	"net/http"

	"distance-matrix/internal/logic"
	"distance-matrix/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func sdksHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewSdksLogic(r.Context(), svcCtx)
		resp, err := l.Sdks()
		if err != nil {
			httpx.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
