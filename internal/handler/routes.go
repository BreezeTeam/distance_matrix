package handler

import (
	"net/http"

	"distance-matrix/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/v1/matrix",
				Handler: matrixHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/v1/route",
				Handler: routeHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/v1/providers",
				Handler: providersHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/health/live",
				Handler: healthLiveHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/health/ready",
				Handler: healthReadyHandler(serverCtx),
			},
		},
	)
}
