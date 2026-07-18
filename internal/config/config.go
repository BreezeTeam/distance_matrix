package config

import (
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Redis       RedisConf       `json:",optional"`
	Persistence PersistenceConf `json:",optional"`
	Engine      EngineConf      `json:",optional"`
	Providers   ProvidersConf   `json:",optional"`
}

// ForServer splits Timeout into:
//   - svcCfg.Timeout: business deadline used by matrix handler → 504 MATRIX_DEADLINE
//   - restConf.Timeout=0: disables go-zero TimeoutHandler (503 "Request Timeout")
//
// go-zero's TimeoutHandler races the handler and returns plain 503 before we can
// emit JSON 504 with write-through semantics. See rest/handler/timeouthandler.go.
func ForServer(c Config) (svcCfg Config, restConf rest.RestConf) {
	svcCfg = c
	if svcCfg.Timeout <= 0 {
		svcCfg.Timeout = 30000
	}
	restConf = c.RestConf
	restConf.Timeout = 0
	return svcCfg, restConf
}

type RedisConf struct {
	Addr    string `json:",default=127.0.0.1:6379"`
	Prefix  string `json:",default=distance_matrix"`
	EdgeTTL int    `json:",default=1209600"`
	Enabled bool   `json:",default=true"`
}

// PersistenceConf: non-empty DSN enables MySQL L2 archive; empty = off.
type PersistenceConf struct {
	DSN          string `json:",optional"`
	Database     string `json:",optional"`
	MaxOpenConns int    `json:",default=10"`
	MaxIdleConns int    `json:",default=5"`
	AsyncQueue   int    `json:",default=1024"`
	AutoMigrate  bool   `json:",default=false"` // GORM AutoMigrate; prefer offline genddl when no CREATE priv
}

type EngineConf struct {
	DefaultGeoWideM int `json:",default=200"`
	MaxPoints       int `json:",default=100"`
	TenantQPS       int `json:",default=50"`
}

type ProvidersConf struct {
	Amap AmapProviderConf `json:",optional"`
}

type AmapProviderConf struct {
	Enabled              bool    `json:",default=true"`
	Keys                 string  `json:",optional"`
	BaseURL              string  `json:",default=http://restapi.amap.com"`
	BatchSize            int     `json:",default=12"`
	TimeoutSec           int     `json:",default=2"`
	KeyRecoverySec       int     `json:",default=300"`
	KeyProbeSec          int     `json:",default=30"`
	KeyConfidenceTau     float64 `json:",default=2"`
	KeyBetaPriorA        float64 `json:",default=2"`
	KeyBetaPriorB        float64 `json:",default=1"`
	KeyFailureSoftWeight float64 `json:",default=0.3"`
	KeyEpsilonScale      float64 `json:",default=4"`
	KeyMinProbeRate      float64 `json:",default=0.02"`
}
