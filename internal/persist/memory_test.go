package persist

import (
	"context"
	"testing"
	"time"

	"distance-matrix/internal/cache"
)

func TestMemoryUpsertGet(t *testing.T) {
	m := NewMemory()
	o := [2]float32{116.40, 39.90}
	d := [2]float32{116.41, 39.91}
	wmt := cache.TimeSlotWMH(time.Date(2026, 7, 17, 14, 0, 0, 0, time.Local))
	opts := cache.LookupOpts{Tenant: "t", Method: 0, Strategy: 0, TimeSlot: wmt, Strict: true}

	if err := m.Upsert(context.Background(), opts, cache.Edge{
		Origin: o, Destination: d, DistanceM: 1200, DurationS: 180, WMT: wmt, Provider: "amap",
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := m.Get(context.Background(), opts, o, d)
	if err != nil || !ok {
		t.Fatalf("expected hit, ok=%v err=%v", ok, err)
	}
	if got.DistanceM != 1200 || got.DurationS != 180 {
		t.Fatalf("got %+v", got)
	}
}

func TestMemorySkipZeroDistance(t *testing.T) {
	m := NewMemory()
	o := [2]float32{116.40, 39.90}
	opts := cache.LookupOpts{Tenant: "t"}
	_ = m.Upsert(context.Background(), opts, cache.Edge{Origin: o, Destination: o, DistanceM: 0, WMT: "5141414"})
	_, ok, _ := m.Get(context.Background(), cache.LookupOpts{Tenant: "t", TimeSlot: "5141414", Strict: true}, o, o)
	if ok {
		t.Fatal("zero distance should not be stored")
	}
}

func TestAsyncEnqueue(t *testing.T) {
	mem := NewMemory()
	async := NewAsyncWriter(mem, 8)
	defer async.Close()

	o := [2]float32{116.40, 39.90}
	d := [2]float32{116.41, 39.91}
	wmt := "5141414"
	opts := cache.LookupOpts{Tenant: "async", TimeSlot: wmt, Strict: true}
	async.EnqueueUpsert(opts, cache.Edge{Origin: o, Destination: d, DistanceM: 99, DurationS: 9, WMT: wmt})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, ok, _ := mem.Get(context.Background(), opts, o, d)
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("async upsert did not land")
}

func TestDatabaseFromDSN(t *testing.T) {
	if got := databaseFromDSN("u:p@tcp(127.0.0.1:3306)/distance_matrix?parseTime=true"); got != "distance_matrix" {
		t.Fatalf("got %q", got)
	}
	if got := databaseFromDSN("u:p@tcp(127.0.0.1:3306)/?parseTime=true"); got != "" {
		t.Fatalf("got %q", got)
	}
}
