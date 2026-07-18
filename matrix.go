package main

import (
	"flag"
	"fmt"
	"runtime"

	"distance-matrix/internal/config"
	"distance-matrix/internal/handler"
	"distance-matrix/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/matrix.yaml", "config file")

func main() {
	flag.Parse()
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// Timeout in yaml is the business deadline (504). Do not pass it to go-zero RestConf,
	// or TimeoutHandler will win the race with 503 "Request Timeout".
	svcCfg, restConf := config.ForServer(c)
	ctx := svc.NewServiceContext(svcCfg)
	defer ctx.Close()

	server := rest.MustNewServer(restConf)
	defer server.Stop()
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
