package cache

import (
	"testing"
	"time"
)

func TestTimeSlotWMH(t *testing.T) {
	tm := time.Date(2026, 7, 15, 14, 30, 0, 0, time.FixedZone("CST", 8*3600))
	slot := TimeSlotWMH(tm)
	// Go weekday: Wed=3 → slot prefix "4"; month=07; hour=14
	if slot != "40714" {
		t.Fatalf("TimeSlotWMH = %q, want 40714", slot)
	}
}

func TestAdjacentSlots(t *testing.T) {
	slots := AdjacentSlots("40714")
	if len(slots) != 3 {
		t.Fatalf("expected 3 slots, got %v", slots)
	}
	want := map[string]bool{"40713": true, "40714": true, "40715": true}
	for _, s := range slots {
		if !want[s] {
			t.Fatalf("unexpected adjacent slot %q in %v", s, slots)
		}
	}
}

func TestAdjacentSlotsHourWrap(t *testing.T) {
	slots := AdjacentSlots("40700")
	has23 := false
	has01 := false
	for _, s := range slots {
		if s == "40723" {
			has23 = true
		}
		if s == "40701" {
			has01 = true
		}
	}
	if !has23 || !has01 {
		t.Fatalf("hour wrap failed: %v", slots)
	}
}

func TestParseEdgeRoundTrip(t *testing.T) {
	e := Edge{
		Origin: [2]float32{116.4, 39.9}, Destination: [2]float32{116.41, 39.91},
		DistanceM: 100, DurationS: 10, WMT: "40714", Provider: "amap",
	}
	parsed, err := ParseEdge(e.JSON())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.DistanceM != 100 || parsed.Provider != "amap" {
		t.Fatalf("round trip failed: %+v", parsed)
	}
}
