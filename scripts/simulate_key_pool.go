// Key pool scheduler simulation — compares presets against live-calibrated QPS fail rate.
//
//	go run ./scripts/simulate_key_pool.go
//	go run ./scripts/simulate_key_pool.go -live   # probe good key QPS fail rate first
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"distance-matrix/internal/loadbalance"
)

const (
	keyGoodPrefix = "9e12a1bc"
	keyDead1      = "c70e3ecb"
	keyDead2      = "4396fb03"
	probeBase     = 30 * time.Second
	reqInterval   = 100 * time.Millisecond // virtual 10 QPS per worker tick
)

var (
	defaultQPSFail = 0.683 // 2026-07-16 live: 30并发×60 OK=19 QPS=41
)

type keyModel struct {
	name       string
	alwaysFail bool
	qpsFailPct float64
}

type simClock struct {
	mu sync.Mutex
	t  time.Time
}

type variant struct {
	name string
	cfg  loadbalance.SchedulerConfig
}

func main() {
	live := flag.Bool("live", false, "probe live Amap good key for QPS fail rate")
	flag.Parse()

	qpsFail := defaultQPSFail
	if *live {
		rate, note := probeGoodKeyQPSFail()
		qpsFail = rate
		fmt.Println("【实测校准】", note)
		fmt.Printf("  好 key QPS 失败率 = %.1f%%  (用于场景模拟)\n\n", qpsFail*100)
	} else {
		fmt.Printf("【参数来源】默认 QPS fail=%.1f%%（最近一次 live）；加 -live 重跑\n\n", defaultQPSFail*100)
	}

	halfLife := 300 * time.Second
	variants := []variant{
		{"Old Min Probe", loadbalance.LegacyMinProbePreset()},
		{"v3 Zeroed", loadbalance.ZeroedPreset()},
		{"v4 ADCS (推荐)", tunedConfig(qpsFail)},
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  重跑模拟结果 — 基于真实 key 画像 + 可调 ADCS 参数               ║")
	fmt.Printf("║  好 key QPS fail=%.1f%%  T₀=%v  half-life=%v  10 QPS/worker      ║\n", qpsFail*100, probeBase, halfLife)
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	for _, v := range variants {
		fmt.Printf("━━ %s ━━\n", v.name)
		printScenario2(v, halfLife, qpsFail)
		printScenario3(v, halfLife, qpsFail)
		if v.name == "v4 ADCS (推荐)" {
			printSingleKey(v, halfLife, qpsFail)
		}
		fmt.Println()
	}

	if v := tunedConfig(qpsFail); true {
		fmt.Println("━━ τ 扫描 (v4 preset, 场景2 成功率) ━━")
		for _, tau := range []float64{0.5, 1, 2, 3, 5} {
			c := v
			c.ConfidenceTau = tau
			ok, _, _, _, _ := runScenario2Once(c, halfLife, qpsFail)
			fmt.Printf("  τ=%.1f → 成功率 %.1f%%\n", tau, pct(ok, 5000))
		}
	}
}

func tunedConfig(qpsFail float64) loadbalance.SchedulerConfig {
	c := loadbalance.DefaultSchedulerConfig()
	// API key pool: fast reject bad keys, slow confidence ramp (τ=2 from sweep)
	c.ConfidenceTau = 2.0
	c.BetaPriorA = 2.0
	c.BetaPriorB = 1.0
	c.FailureSoftWeight = 0.3
	c.EpsilonScale = 4.0
	c.MinProbeRate = 0.02
	_ = qpsFail // reserved for future per-provider tuning
	return c
}

func newSimPool(halfLife time.Duration, cfg loadbalance.SchedulerConfig) (*loadbalance.Pool, *simClock) {
	clock := &simClock{t: time.Now()}
	pool := loadbalance.NewPoolWithConfig(halfLife, probeBase, cfg)
	pool.SetVirtualNow(func() time.Time {
		clock.mu.Lock()
		defer clock.mu.Unlock()
		return clock.t
	})
	return pool, clock
}

func printScenario2(v variant, halfLife time.Duration, qpsFail float64) {
	ok, fb, keys, pool, clock := runScenario2Once(v.cfg, halfLife, qpsFail)
	fmt.Println("  场景2: 3 key (1 好 + 2 永久坏) 5000 请求")
	fmt.Printf("    成功率 %.1f%% | fallback %.1f%%\n", pct(ok, 5000), pct(fb, 5000))
	fmt.Printf("    好 key 权重 %.1f%% | 坏 key 各 %.1f%% (持续探测)\n",
		share(pool, clock, keyGoodPrefix, 3000),
		(share(pool, clock, keyDead1, 3000)+share(pool, clock, keyDead2, 3000))/2)
	_ = keys
}

func runScenario2Once(cfg loadbalance.SchedulerConfig, halfLife time.Duration, qpsFail float64) (ok, fb int, keys []keyModel, pool *loadbalance.Pool, clock *simClock) {
	keys = []keyModel{
		{keyGoodPrefix, false, qpsFail},
		{keyDead1, true, 0},
		{keyDead2, true, 0},
	}
	pool, clock = newSimPool(halfLife, cfg)
	for _, k := range keys {
		pool.Add(k.name, 1)
	}
	ok, fb, _ = runTraffic(pool, clock, keys, 50, 5000)
	return ok, fb, keys, pool, clock
}

func printScenario3(v variant, halfLife time.Duration, qpsFail float64) {
	pool, clock := newSimPool(halfLife, v.cfg)
	pool.Add(keyGoodPrefix, 1)
	pool.Add(keyDead1, 1)
	pool.Add(keyDead2, 1)

	for i := 0; i < 10; i++ {
		pool.RecordSuccess(keyGoodPrefix)
	}
	for i := 0; i < 40; i++ {
		pool.RecordFailure(keyGoodPrefix) // soft: QPS 类失败，非永久坏 key
	}
	afterBurst := pool.EffectiveRateAfter(keyGoodPrefix, probeBase) * 100

	keys := []keyModel{
		{keyGoodPrefix, false, qpsFail},
		{keyDead1, true, 0},
		{keyDead2, true, 0},
	}
	ok, fb, _ := runTraffic(pool, clock, keys, 20, 200)

	fmt.Println("  场景3: 好 key 连续 40 次失败")
	fmt.Printf("    失败后 R@30s=%.2f%% | 随后200请求 成功 %.1f%% fallback %.1f%%\n",
		afterBurst, pct(ok, 200), pct(fb, 200))
	fmt.Printf("    稳态 好 key %.1f%% | 坏 key1 %.1f%% | 坏 key2 %.1f%%\n",
		share(pool, clock, keyGoodPrefix, 2000),
		share(pool, clock, keyDead1, 2000),
		share(pool, clock, keyDead2, 2000))
}

func printSingleKey(v variant, halfLife time.Duration, qpsFail float64) {
	pool, clock := newSimPool(halfLife, v.cfg)
	pool.Add(keyGoodPrefix, 1)
	keys := []keyModel{{keyGoodPrefix, false, qpsFail}}
	ok, fb, _ := runTraffic(pool, clock, keys, 50, 5000)
	fmt.Println("  对照: 仅 1 个好 key")
	fmt.Printf("    成功率 %.1f%% | fallback %.1f%% (单 key ε=e^{-N/4}≈%.2f)\n",
		pct(ok, 5000), pct(fb, 5000), mathExp(-1.0/4.0))
}

func runTraffic(pool *loadbalance.Pool, clock *simClock, keys []keyModel, workers, totalReq int) (ok, fallback, qpsFail int) {
	var mu sync.Mutex
	try := func() {
		clock.tick(reqInterval)
		tried := map[string]struct{}{}
		for len(tried) < pool.Len() {
			key, _ := pool.Next().(string)
			if key == "" {
				break
			}
			if _, seen := tried[key]; seen {
				break
			}
			tried[key] = struct{}{}

			fail := false
			for _, k := range keys {
				if k.name != key {
					continue
				}
				if k.alwaysFail {
					fail = true
				} else if rand.Float64() < k.qpsFailPct {
					fail = true
					mu.Lock()
					qpsFail++
					mu.Unlock()
				}
				break
			}
			if fail {
				pool.RecordFailure(key)
				continue
			}
			pool.RecordSuccess(key)
			mu.Lock()
			ok++
			mu.Unlock()
			return
		}
		mu.Lock()
		fallback++
		mu.Unlock()
	}

	rand.Seed(42)
	var wg sync.WaitGroup
	perWorker := totalReq / workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				try()
			}
		}()
	}
	wg.Wait()
	return ok, fallback, qpsFail
}

func (c *simClock) tick(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func share(p *loadbalance.Pool, clock *simClock, key string, n int) float64 {
	return pct(count(p, clock, key, n), n)
}

func count(p *loadbalance.Pool, clock *simClock, key string, n int) int {
	c := 0
	for i := 0; i < n; i++ {
		clock.tick(reqInterval)
		if k, _ := p.Next().(string); strings.HasPrefix(k, key) || k == key {
			c++
		}
	}
	return c
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b) * 100
}

func mathExp(x float64) float64 {
	// tiny helper to avoid importing math in one printf
	const (
		a = 1.0
	)
	e := a
	term := a
	for i := 1; i < 12; i++ {
		term *= x / float64(i)
		e += term
	}
	return e
}

func probeGoodKeyQPSFail() (float64, string) {
	configPath := "etc/matrix.yaml"
	if _, err := os.Stat(configPath); err != nil {
		return defaultQPSFail, "etc/matrix.yaml 不可用，使用默认"
	}
	text, _ := os.ReadFile(configPath)
	re := regexp.MustCompile(`Keys:\s*(.+)`)
	m := re.FindSubmatch(text)
	if len(m) < 2 {
		return defaultQPSFail, "未解析 Keys"
	}
	var goodKey string
	for _, k := range strings.Split(string(m[1]), ",") {
		k = strings.TrimSpace(k)
		if strings.HasPrefix(k, keyGoodPrefix) {
			goodKey = k
			break
		}
	}
	if goodKey == "" {
		return defaultQPSFail, "未找到好 key"
	}

	const n = 60
	var ok, qps int
	var wg sync.WaitGroup
	sem := make(chan struct{}, 30)
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fail, isQPS := callAmap(goodKey)
			if fail {
				if isQPS {
					qps++
				}
			} else {
				ok++
			}
		}()
	}
	wg.Wait()
	failRate := 1.0 - float64(ok)/float64(n)
	return failRate, fmt.Sprintf("30 并发 × %d 次  OK=%d QPS类失败=%d", n, ok, qps)
}

func callAmap(key string) (failed, qpsThrottle bool) {
	q := url.Values{}
	q.Set("key", key)
	q.Set("origin", "116.397428,39.90923")
	q.Set("destination", "116.407428,39.91923")
	q.Set("strategy", "11")
	q.Set("output", "json")
	resp, err := http.Get("http://restapi.amap.com/v3/direction/driving?" + q.Encode())
	if err != nil {
		return true, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var v struct {
		Status   string `json:"status"`
		Infocode string `json:"infocode"`
	}
	_ = json.Unmarshal(body, &v)
	if v.Status == "1" {
		return false, false
	}
	return true, v.Infocode == "10021"
}
