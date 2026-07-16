package config

import (
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Redis     RedisConf     `json:",optional"`
	Engine    EngineConf    `json:",optional"`
	Providers ProvidersConf `json:",optional"`
}

type RedisConf struct {
	Addr    string `json:",default=127.0.0.1:6379"`
	Prefix  string `json:",default=distance_matrix"`
	EdgeTTL int    `json:",default=1209600"`
	Enabled bool   `json:",default=true"`
}

type EngineConf struct {
	DefaultGeoWideM int     `json:",default=200"`
	MaxPoints       int     `json:",default=100"`
	FallbackFactor  float32 `json:",default=1.5"`
	TenantQPS       int     `json:",default=50"`
}

type ProvidersConf struct {
	Amap AmapProviderConf `json:",optional"`
}

type AmapProviderConf struct {
	Enabled            bool    `json:",default=true"`
	Keys               string  `json:",optional"`
	BaseURL            string  `json:",default=http://restapi.amap.com"`
	BatchSize          int     `json:",default=12"`
	TimeoutSec         int     `json:",default=2"`
	KeyRecoverySec     int     `json:",default=300"`
	KeyProbeSec        int     `json:",default=30"`
	KeyConfidenceTau   float64 `json:",default=2"`
	KeyBetaPriorA      float64 `json:",default=2"`
	KeyBetaPriorB      float64 `json:",default=1"`
	KeyFailureSoftWeight float64 `json:",default=0.3"`
	KeyEpsilonScale    float64 `json:",default=4"`
	KeyMinProbeRate    float64 `json:",default=0.02"`
}
