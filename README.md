# distance_matrix
Distance Matrix service based on golang and zeromicro

# 地理信息寻址服务

go env 设置 go env -w GOPROXY=https://goproxy.cn/,direct

# go mod

go mod init polaris-matrix

# install go-zero

go get -u github.com/zeromicro/go-zero go get -u github.com/zeromicro/go-zero/tools/goctl

# 代码生成

goctl api go -api common/service.api -dir .

# 安装依赖

go mod tidy

# 基本结构更改

添加配置到config 结构体中,这样就可以在上下文中获取到sdk配置，并实现sdk的自动注册和配置

```go
package config

import (
	"github.com/zeromicro/go-zero/rest"
	"polaris-matrix/common"
)

type Config struct {
	common.ServiceConfig
	rest.RestConf
}

```

# 启动

go run matrix.go -f etc/matrix.yaml

# 可以根据 api 文件生成前端需要的 Java, TypeScript, Dart, JavaScript 代码

goctl api java -api greet.api -dir greet

# swagger doc

go get github.com/zeromicro/goctl-swagger 

goctl api plugin -plugin goctl-swagger="swagger -filename openapi.json" -api common/service.api -dir .

# serve swagger
swagger_windows_amd64.exe serve -F=swagger openapi.json --port 8888 --host 0.0.0.0


# 部署
`docker build -t registry.ztosys.com/lzxt/matrix:v0.0.3 . `
`docker login https://registry.ztosys.com/harbor`
`docker push registry.ztosys.com/lzxt/matrix:v0.0.3`


```curl

curl -X POST --location "http://pro-quantum-matrix:8888/api/route"  -H "Content-Type: application/json"  -d "{ \"coordinate\":\"gcj02\", \"points\":[ [116.223, 39.9057], [116.1747, 39.9437], [116.223, 39.9057], [116.1747, 39.9437] ] }"
```