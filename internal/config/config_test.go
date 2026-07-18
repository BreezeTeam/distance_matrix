package config

import "testing"

func TestForServerDisablesRestTimeout(t *testing.T) {
	c := Config{}
	c.Timeout = 1500
	c.Name = "matrix"
	svcCfg, restConf := ForServer(c)
	if svcCfg.Timeout != 1500 {
		t.Fatalf("business timeout=%d", svcCfg.Timeout)
	}
	if restConf.Timeout != 0 {
		t.Fatalf("rest timeout should be 0 to disable go-zero TimeoutHandler, got %d", restConf.Timeout)
	}
}

func TestForServerDefaultBusinessTimeout(t *testing.T) {
	svcCfg, restConf := ForServer(Config{})
	if svcCfg.Timeout != 30000 {
		t.Fatalf("default business timeout=%d", svcCfg.Timeout)
	}
	if restConf.Timeout != 0 {
		t.Fatalf("rest timeout=%d", restConf.Timeout)
	}
}
