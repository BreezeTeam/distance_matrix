package handler

import (
	"context"
	"errors"
	"net/http"
	"time"
)

func tenantID(r *http.Request) string {
	if t := r.Header.Get("X-Tenant-Id"); t != "" {
		return t
	}
	return "default"
}

func parsePoints(raw [][]float32) ([][2]float32, error) {
	points := make([][2]float32, 0, len(raw))
	for _, p := range raw {
		if len(p) < 2 {
			return nil, errors.New("invalid point")
		}
		points = append(points, [2]float32{p[0], p[1]})
	}
	return points, nil
}

func contextWithDeadline(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, d)
}
