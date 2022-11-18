// Package common
// @Author Euraxluo  17:18:00
package common

import (
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"quantum-matrix/sdk"
	_ "quantum-matrix/sdk/base"
)

// ServiceConfig  此处为服务配置，和api 无关
type ServiceConfig struct {
	// logx.LogConf  此为日志配置，使用的是logx
	Log logx.LogConf
	// SDK 配置
	SDK map[string]map[string]interface{}
}

var GlobalConfig *ServiceConfig = &ServiceConfig{
	SDK: make(map[string]map[string]interface{}),
}

// InitConfig  初始化配置
// 1. 获取sdk配置定义，解析配置并注入到全局配置中
// 2. 加载配置，并反向注入回sdk中
func InitConfig(filename string) error {
	//  从文件解析配置,go-zero 配置优先，如果没有配置的情况，会覆盖为nil
	conf.MustLoad(filename, GlobalConfig, conf.UseEnv())
	//  sdk配置解析
	for k, v := range sdk.OptionFactoryByName {
		if _, ok := GlobalConfig.SDK[k]; ok {
			//  已经解析了，应当只补充，没有的值
			defaultOpt := v.Map()
			for key, value := range defaultOpt {
				if _, optHas := GlobalConfig.SDK[k][key]; !optHas {
					//  已经解析了，应当只补充，没有的值
					GlobalConfig.SDK[k][key] = value
				}
			}
		} else {
			//  没有，直接添加
			GlobalConfig.SDK[k] = v.Map()
		}
		logx.Infof("SDK %s config resolution to map success", k)
	}
	logx.Infof("config parse from %s success", filename)
	// logx 根据配置初始化
	logx.MustSetup(GlobalConfig.Log)
	logx.Infof("logx init success")
	//  反向注入回sdk
	for k, v := range GlobalConfig.SDK {
		sdk.OptionFactoryByName[k].Update(v)
		logx.Infof("SDK config injection to sdk.OptionFactoryByName: %s ,config: %v", k, v)
	}
	return nil
}
