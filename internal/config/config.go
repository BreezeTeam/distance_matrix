package config

import (
	"github.com/zeromicro/go-zero/rest"
	"quantum-matrix/common"
)

type Config struct {
	common.ServiceConfig
	rest.RestConf
}
