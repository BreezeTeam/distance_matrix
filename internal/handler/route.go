package handler

import (
	"errors"
	"net/http"

	"distance-matrix/internal/provider"
	"distance-matrix/internal/svc"
	"distance-matrix/internal/types"
	"distance-matrix/internal/geo"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func routeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RouteRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Error(w, err)
			return
		}
		raw, err := parsePoints(req.Points)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		if len(raw) < 2 {
			httpx.Error(w, errors.New("need at least 2 points"))
			return
		}

		waypoints := geo.CoordConvertToGCJ02(toSlice(raw), req.Coordinate)
		prov, err := svcCtx.Registry.Get(req.Provider)
		if err != nil {
			httpx.Error(w, err)
			return
		}

		steps, _, err := svcCtx.Planner.RouteWaypoints(r.Context(), prov, provider.RouteRequest{
			Strategy: req.Strategy,
			Method:   req.Method,
			SpeedMPS: req.SpeedMPS,
		}, waypoints)
		if err != nil {
			httpx.Error(w, err)
			return
		}

		data := &types.RouteData{Steps: make([]types.RouteStep, 0, len(steps))}
		for _, st := range steps {
			data.Steps = append(data.Steps, types.RouteStep{
				Origin:      []float32{st.Origin[0], st.Origin[1]},
				Destination: []float32{st.Destination[0], st.Destination[1]},
				Distance:    st.DistanceM,
				Duration:    st.DurationS,
			})
			data.Distance += st.DistanceM
			data.Duration += st.DurationS
		}

		httpx.OkJson(w, types.RouteResponse{Code: 200, Msg: "OK", Data: data})
	}
}

func toSlice(in [][2]float32) [][]float32 {
	out := make([][]float32, len(in))
	for i, p := range in {
		out[i] = []float32{p[0], p[1]}
	}
	return out
}
