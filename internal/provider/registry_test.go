package provider

import (
	"context"
	"testing"
)

type fakeProvider struct {
	name  string
	ready bool
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) Ready() bool  { return f.ready }
func (f fakeProvider) Route(context.Context, RouteRequest) (*RouteResult, error) {
	return &RouteResult{}, nil
}

func TestRegistryGetDefault(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeProvider{name: "amap", ready: true})
	reg.Register(fakeProvider{name: "other", ready: true})

	p, err := reg.Get("")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "amap" {
		t.Fatalf("default provider = %q", p.Name())
	}

	reg.SetDefault("other")
	p, err = reg.Get("")
	if err != nil || p.Name() != "other" {
		t.Fatalf("SetDefault failed: %v %q", err, p.Name())
	}
}

func TestRegistryGetMissing(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeProvider{name: "amap", ready: true})
	if _, err := reg.Get("missing"); err == nil {
		t.Fatal("expected error for missing provider")
	}
}

func TestRegistryReady(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeProvider{name: "down", ready: false})
	if reg.Ready() {
		t.Fatal("should not be ready")
	}
	reg.Register(fakeProvider{name: "up", ready: true})
	if !reg.Ready() {
		t.Fatal("should be ready when any provider is ready")
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeProvider{name: "amap", ready: true})
	names := reg.List()
	if len(names) != 1 || names[0] != "amap" {
		t.Fatalf("list = %v", names)
	}
}
