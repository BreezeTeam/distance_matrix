package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"distance-matrix/internal/engine"
	"distance-matrix/internal/metrics"
	"distance-matrix/internal/middleware"
	"distance-matrix/internal/svc"
	"distance-matrix/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func matrixHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	limit := middleware.TenantRateLimit(svcCtx.Config.Engine.TenantQPS)
	return limit(func(w http.ResponseWriter, r *http.Request) {
		var req types.MatrixRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Error(w, err)
			return
		}
		points, err := parsePoints(req.Points)
		if err != nil {
			httpx.Error(w, err)
			return
		}

		tenant := tenantID(r)
		start := time.Now()
		geoWide := req.GeoWideM
		if geoWide <= 0 {
			geoWide = svcCtx.Config.Engine.DefaultGeoWideM
		}
		if geoWide <= 0 {
			geoWide = 200
		}

		timeoutMs := svcCtx.Config.Timeout
		if timeoutMs <= 0 {
			timeoutMs = 5000
		}
		ctx, cancel := contextWithDeadline(r.Context(), time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()

		result, err := svcCtx.Matrix.Compute(ctx, engine.Request{
			Points:     points,
			Coordinate: req.Coordinate,
			Strategy:   req.Strategy,
			Method:     req.Method,
			TimeSlot:   req.TimeSlot,
			Strict:     req.Strict,
			GeoWideM:   geoWide,
			Provider:   req.Provider,
			SpeedMPS:   req.SpeedMPS,
			Tenant:     tenant,
		})
		if errors.Is(err, engine.ErrDeadline) {
			metrics.RecordMatrix(tenant, "504", time.Since(start).Milliseconds(), 0, 0)
			w.WriteHeader(http.StatusGatewayTimeout)
			_ = json.NewEncoder(w).Encode(types.MatrixResponse{Code: 504, Msg: "MATRIX_DEADLINE"})
			return
		}
		if err != nil {
			httpx.Error(w, err)
			return
		}

		logx.WithContext(r.Context()).Infof(
			"matrix tenant=%s n=%d cache_hit=%.2f hits=%d misses=%d fallback=%d provider_calls=%d arccover_calls=%d elapsed_ms=%d",
			tenant, len(points), result.Stats.CacheHitRatio, result.Stats.CacheHits, result.Stats.CacheMisses,
			result.Stats.FallbackEdges, result.Stats.ProviderCalls, result.Stats.ArcCoverCalls, result.Stats.ElapsedMs,
		)
		metrics.RecordMatrix(tenant, "200", result.Stats.ElapsedMs, result.Stats.FallbackEdges, result.Stats.ProviderCalls)

		httpx.OkJson(w, types.MatrixResponse{
			Code: 200,
			Msg:  "OK",
			Data: &types.MatrixData{Distances: result.Distances, Durations: result.Durations},
		})
	})
}
