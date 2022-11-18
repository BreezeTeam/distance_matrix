// Package sdk
// @Author Euraxluo  16:52:00
package sdk

import "log"

// RouteRunner  路由运行者
type RouteRunner struct {
	runners []chan *LocationRouteServiceSchema
}

// NewRouteRunner  创建一个运行者，concurrency 是运行者的数量
func NewRouteRunner(concurrency int) *RouteRunner {
	return &RouteRunner{
		runners: make([]chan *LocationRouteServiceSchema, concurrency),
	}
}

// Recover
func Recover(cleanups ...func()) {
	for _, cleanup := range cleanups {
		cleanup()
	}

	if p := recover(); p != nil {
		log.Printf("%v\n", p)
	}
}

// Run  将路由运行函数放入路由运行管道中异步执行
func (rr *RouteRunner) Run(i int, task func() *LocationRouteServiceSchema) {
	rr.runners[i] = make(chan *LocationRouteServiceSchema)
	go func() {
		defer Recover()
		rr.runners[i] <- task()

	}()
}

// Results  结果收集
func (rr *RouteRunner) Results() (results []*LocationRouteServiceSchema) {
	results = make([]*LocationRouteServiceSchema, len(rr.runners))
	for i, ch := range rr.runners {
		results[i] = <-ch
	}
	return
}
