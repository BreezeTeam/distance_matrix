package main

import (
	"distance-matrix/internal/handler"
	"distance-matrix/internal/svc"
	"flag"
	"fmt"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"runtime"

	"distance-matrix/internal/config"
)

func getGOMAXPROCS() int {
	runtime.NumCPU()             // 获取机器的CPU核心数
	return runtime.GOMAXPROCS(0) // 参数为零时用于获取给GOMAXPROCS设置的值
}

var configFile = flag.String("f", "etc/matrix.yaml", "the config file")

func main() {
	fmt.Printf("GOMAXPROCS: %d\n", getGOMAXPROCS())
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	ctx := svc.NewServiceContext(c)
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
