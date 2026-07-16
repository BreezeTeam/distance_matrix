package handler

import (
	"net/http"

	"distance-matrix/internal/svc"
)

// MatrixHTTP exposes the matrix handler for external integration tests.
func MatrixHTTP(svcCtx *svc.ServiceContext) http.HandlerFunc { return matrixHandler(svcCtx) }

// RouteHTTP exposes the route handler for external integration tests.
func RouteHTTP(svcCtx *svc.ServiceContext) http.HandlerFunc { return routeHandler(svcCtx) }
