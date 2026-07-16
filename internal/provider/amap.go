package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"distance-matrix/internal/geo"
	"distance-matrix/internal/loadbalance"
	"github.com/the-go-tool/cast"
)

// AmapConfig configures the Amap driving provider.
type AmapConfig struct {
	Enabled              bool
	Keys                 string
	BaseURL              string
	BatchSize            int
	TimeoutSec           int
	KeyRecoverySec       int
	KeyProbeSec          int
	ConfidenceTau        float64
	BetaPriorA           float64
	BetaPriorB           float64
	FailureSoftWeight    float64
	EpsilonScale         float64
	MinProbeRate         float64
}

func schedulerConfigFromAmap(cfg AmapConfig) loadbalance.SchedulerConfig {
	c := loadbalance.DefaultSchedulerConfig()
	if cfg.ConfidenceTau > 0 {
		c.ConfidenceTau = cfg.ConfidenceTau
	}
	if cfg.BetaPriorA > 0 {
		c.BetaPriorA = cfg.BetaPriorA
	}
	if cfg.BetaPriorB > 0 {
		c.BetaPriorB = cfg.BetaPriorB
	}
	if cfg.FailureSoftWeight > 0 {
		c.FailureSoftWeight = cfg.FailureSoftWeight
	}
	if cfg.EpsilonScale > 0 {
		c.EpsilonScale = cfg.EpsilonScale
	}
	if cfg.MinProbeRate > 0 {
		c.MinProbeRate = cfg.MinProbeRate
	}
	return c
}

// AmapProvider calls Amap v3 direction/driving.
type AmapProvider struct {
	cfg    AmapConfig
	client *http.Client
	lb     *loadbalance.Pool
}

func NewAmapProvider(cfg AmapConfig) *AmapProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://restapi.amap.com"
	}
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 2
	}
	p := &AmapProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSec) * time.Second,
		},
		lb: loadbalance.NewPoolWithConfig(
			time.Duration(cfg.KeyRecoverySec)*time.Second,
			time.Duration(cfg.KeyProbeSec)*time.Second,
			schedulerConfigFromAmap(cfg),
		),
	}
	p.reloadKeys()
	return p
}

func (p *AmapProvider) reloadKeys() {
	p.lb.RemoveAll()
	for _, k := range strings.Split(p.cfg.Keys, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			p.lb.Add(k, 1)
		}
	}
}

func (p *AmapProvider) Name() string { return "amap" }

func (p *AmapProvider) Ready() bool {
	return p.cfg.Enabled && p.lb.All() != nil && len(p.lb.All()) > 0
}

func strategyCode(strategy int) string {
	m := map[int]string{
		geo.Default:          "11",
		geo.ShortestDistance: "12",
		geo.AvoidCongestion:  "1",
		geo.UnWalkFastRoute:  "2",
	}
	if v, ok := m[strategy]; ok {
		return v
	}
	return "11"
}

func (p *AmapProvider) Route(ctx context.Context, req RouteRequest) (*RouteResult, error) {
	if len(req.Waypoints) < 2 {
		return &RouteResult{}, nil
	}
	if !p.Ready() {
		return p.fallbackRoute(req), nil
	}
	attempts := p.lb.Len()
	if attempts == 0 {
		return p.fallbackRoute(req), nil
	}
	tried := make(map[string]struct{}, attempts)
	for len(tried) < attempts {
		key, _ := p.lb.Next().(string)
		if key == "" {
			break
		}
		if _, ok := tried[key]; ok {
			break
		}
		tried[key] = struct{}{}
		res, err := p.doRequest(ctx, key, req)
		if err == nil {
			p.lb.RecordSuccess(key)
			return res, nil
		}
		p.lb.RecordFailure(key)
	}
	return p.fallbackRoute(req), nil
}

func (p *AmapProvider) allKeys() []string {
	items := p.lb.ActiveItems()
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if k, ok := item.(string); ok {
			keys = append(keys, k)
		}
	}
	return keys
}

func (p *AmapProvider) doRequest(ctx context.Context, key string, req RouteRequest) (*RouteResult, error) {
	wps := req.Waypoints
	origin := wps[0]
	dest := wps[len(wps)-1]
	mid := wps[1 : len(wps)-1]

	q := url.Values{}
	q.Set("key", key)
	q.Set("origin", fmt.Sprintf("%f,%f", origin[0], origin[1]))
	q.Set("destination", fmt.Sprintf("%f,%f", dest[0], dest[1]))
	if len(mid) > 0 {
		q.Set("waypoints", geo.FloatEEToPolyline(mid, ";"))
	}
	q.Set("strategy", strategyCode(req.Strategy))
	q.Set("output", "json")

	u, _ := url.Parse(p.cfg.BaseURL)
	u.Path = path.Join(u.Path, "/v3/direction/driving")
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("amap http %d", resp.StatusCode)
	}

	var v amapResponse
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	if v.Status != "1" || len(v.Route.Paths) == 0 {
		return nil, fmt.Errorf("amap api %s-%s", v.Infocode, v.Info)
	}
	return v.toResult(wps), nil
}

func (p *AmapProvider) fallbackRoute(req RouteRequest) *RouteResult {
	speed := float32(geo.Max(req.SpeedMPS, 7))
	factor := float32(1.5)
	var steps []Step
	var totalD, totalT float32
	for i := 0; i+1 < len(req.Waypoints); i++ {
		a, b := req.Waypoints[i], req.Waypoints[i+1]
		d := geo.Distance[float32](a[0], a[1], b[0], b[1]) * factor
		t := d / speed
		totalD += d
		totalT += t
		steps = append(steps, Step{
			Origin: a, Destination: b,
			DistanceM: d, DurationS: t,
			Source: SourceFallback,
		})
	}
	return &RouteResult{Steps: steps, DistanceM: totalD, DurationS: totalT, Source: SourceFallback, Degraded: true}
}

type amapResponse struct {
	Status   string `json:"status"`
	Info     string `json:"info"`
	Infocode string `json:"infocode"`
	Route    struct {
		Paths []struct {
			Distance string `json:"distance"`
			Duration string `json:"duration"`
			Steps    []struct {
				Distance        string      `json:"distance"`
				Duration        string      `json:"duration"`
				Polyline        string      `json:"polyline"`
				AssistantAction interface{} `json:"assistant_action"`
			} `json:"steps"`
		} `json:"paths"`
	} `json:"route"`
}

func (r amapResponse) toResult(waypoints [][2]float32) *RouteResult {
	path := r.Route.Paths[0]
	var steps []Step
	checkPoint := 1
	for _, k := range path.Steps {
		cur := 0
		if val, ok := k.AssistantAction.(string); ok && geo.In(val, []string{"到达途经地", "到达目的地"}) {
			cur = 1
		}
		if checkPoint*(checkPoint|cur) == 0 && len(steps) > 0 {
			last := steps[len(steps)-1]
			last.DistanceM += cast.To[float32](k.Distance)
			last.DurationS += cast.To[float32](k.Duration)
			steps[len(steps)-1] = last
		} else if len(steps)+1 < len(waypoints) {
			steps = append(steps, Step{
				Origin:      waypoints[len(steps)],
				Destination: waypoints[len(steps)+1],
				DistanceM:   cast.To[float32](k.Distance),
				DurationS:   cast.To[float32](k.Duration),
				Polyline:    k.Polyline,
				Source:      SourceProvider,
			})
		}
		checkPoint = cur
	}
	return &RouteResult{
		Steps:     steps,
		DistanceM: cast.To[float32](path.Distance),
		DurationS: cast.To[float32](path.Duration),
		Source:    SourceProvider,
	}
}
