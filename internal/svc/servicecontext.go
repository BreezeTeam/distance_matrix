package svc

import (
	"context"
	"strings"

	"distance-matrix/internal/arccover"
	"distance-matrix/internal/cache"
	"distance-matrix/internal/config"
	"distance-matrix/internal/engine"
	"distance-matrix/internal/persist"
	"distance-matrix/internal/planner"
	"distance-matrix/internal/provider"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config   config.Config
	Redis    *redis.Client
	Cache    *cache.Store
	Archive  persist.Archive
	Async    *persist.AsyncWriter
	Registry *provider.Registry
	Matrix   *engine.Engine
	Planner  *planner.Planner
}

func NewServiceContext(c config.Config) *ServiceContext {
	ctx := &ServiceContext{Config: c}

	reg := provider.NewRegistry()
	amapCfg := provider.AmapConfig{
		Enabled:           c.Providers.Amap.Enabled,
		Keys:              c.Providers.Amap.Keys,
		BaseURL:           c.Providers.Amap.BaseURL,
		BatchSize:         c.Providers.Amap.BatchSize,
		TimeoutSec:        c.Providers.Amap.TimeoutSec,
		KeyRecoverySec:    c.Providers.Amap.KeyRecoverySec,
		KeyProbeSec:       c.Providers.Amap.KeyProbeSec,
		ConfidenceTau:     c.Providers.Amap.KeyConfidenceTau,
		BetaPriorA:        c.Providers.Amap.KeyBetaPriorA,
		BetaPriorB:        c.Providers.Amap.KeyBetaPriorB,
		FailureSoftWeight: c.Providers.Amap.KeyFailureSoftWeight,
		EpsilonScale:      c.Providers.Amap.KeyEpsilonScale,
		MinProbeRate:      c.Providers.Amap.KeyMinProbeRate,
	}
	reg.Register(provider.NewAmapProvider(amapCfg))
	reg.SetDefault("amap")
	ctx.Registry = reg

	batch := c.Providers.Amap.BatchSize
	if batch <= 0 {
		batch = 12
	}
	ctx.Planner = planner.NewPlanner(batch)
	ctx.Matrix = &engine.Engine{
		Planner:   ctx.Planner,
		Registry:  reg,
		ArcCover:  arccover.NewDensePlanner(arccover.DefaultConfig()),
		MaxPoints: c.Engine.MaxPoints,
	}

	if c.Redis.Enabled {
		ctx.Redis = redis.NewClient(&redis.Options{Addr: c.Redis.Addr})
		if err := ctx.Redis.Ping(context.Background()).Err(); err != nil {
			logx.Errorf("redis ping failed: %v (running without cache)", err)
		} else {
			ctx.Cache = cache.NewStore(ctx.Redis, c.Redis.Prefix, c.Redis.EdgeTTL)
			ctx.Matrix.Cache = ctx.Cache
			logx.Infof("redis cache enabled addr=%s prefix=%s", c.Redis.Addr, c.Redis.Prefix)
		}
	}

	// Persistence: non-empty DSN auto-enables L2 archive + async write-through.
	if dsn := strings.TrimSpace(c.Persistence.DSN); dsn != "" {
		store, err := persist.Open(persist.OpenOptions{
			DSN:          dsn,
			Database:     c.Persistence.Database,
			MaxOpenConns: c.Persistence.MaxOpenConns,
			MaxIdleConns: c.Persistence.MaxIdleConns,
			AutoMigrate:  c.Persistence.AutoMigrate,
		})
		if err != nil {
			logx.Errorf("persistence open failed: %v (running without edge archive)", err)
		} else {
			async := persist.NewAsyncWriter(store, c.Persistence.AsyncQueue)
			ctx.Archive = store
			ctx.Async = async
			ctx.Matrix.Archive = store
			ctx.Matrix.Async = async
			logx.Infof("edge archive enabled (MySQL L2 + async upsert, auto_migrate=%v)", c.Persistence.AutoMigrate)
		}
	}

	return ctx
}

func (s *ServiceContext) Ready() bool {
	if s.Config.Redis.Enabled && s.Cache == nil {
		return false
	}
	return s.Registry.Ready()
}

func (s *ServiceContext) Close() {
	if s.Async != nil {
		_ = s.Async.Close()
		return
	}
	if s.Archive != nil {
		_ = s.Archive.Close()
	}
}
