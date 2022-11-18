package common

import (
	"flag"
	"github.com/zeromicro/go-zero/core/logx"
	"os"
	"runtime"
)

var (
	//  配置 文件路径
	configFile string
)

// initArgs  设置和解析命令行参数
func initArgs() error {
	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	f.StringVar(&configFile, "f", "etc/matrix.yaml", "the config file")
	return f.Parse(os.Args[1:])
}

// initEnv  初始化golang环境
func initEnv() error {
	runtime.GOMAXPROCS(runtime.NumCPU())
	return nil
}
func init() {
	var err error
	//  配置golang环境
	if err = initEnv(); err != nil {
		goto ERR
	}

	//  解析命令行参数
	if err = initArgs(); err != nil {
		goto ERR
	}

	//  配置解析
	if err = InitConfig(configFile); err != nil {
		goto ERR
	}
	return
ERR:
	logx.Error(err.Error())
}
