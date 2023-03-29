package config

import (
	"distance-matrix/common"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	common.ServiceConfig
	rest.RestConf
}
