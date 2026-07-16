package geo

import (
	"math"
	"testing"
)

func TestCoordConvertToGCJ02Identity(t *testing.T) {
	points := [][]float32{{116.40, 39.90}, {116.41, 39.91}}
	out := CoordConvertToGCJ02(points, GCJ02)
	if len(out) != 2 || out[0][0] != points[0][0] || out[1][1] != points[1][1] {
		t.Fatalf("gcj02 identity failed: %v", out)
	}
}

func TestWGS84toGCJ02InChina(t *testing.T) {
	lon, lat := WGS84toGCJ02[float32](116.397128, 39.916527)
	if lon == 116.397128 && lat == 39.916527 {
		t.Fatal("WGS84 in China should offset coordinates")
	}
	if !InChina(lon, lat) {
		t.Fatal("converted point should remain in China bbox")
	}
}

func TestWGS84toGCJ02OutsideChina(t *testing.T) {
	lon, lat := WGS84toGCJ02[float32](0, 0)
	if lon != 0 || lat != 0 {
		t.Fatalf("outside China should pass through: %f,%f", lon, lat)
	}
}

func TestDistanceKnownPair(t *testing.T) {
	// Same point → zero distance.
	d := Distance[float32](116.4, 39.9, 116.4, 39.9)
	if d != 0 {
		t.Fatalf("same point distance = %f", d)
	}
	// Roughly 1.4 km for ~0.01° longitude at this latitude.
	d = Distance[float32](116.40, 39.90, 116.41, 39.90)
	if d < 800 || d > 2000 {
		t.Fatalf("unexpected distance %f", d)
	}
}

func TestWaypointsPacketOverlap(t *testing.T) {
	wps := [][2]float32{{1, 1}, {2, 2}, {3, 3}, {4, 4}, {5, 5}}
	groups := WaypointsPacket(3, wps...)
	if len(groups) < 2 {
		t.Fatalf("expected multiple packets, got %d", len(groups))
	}
	// Each packet after the first should overlap previous endpoint.
	for i := 1; i < len(groups); i++ {
		prevLast := groups[i-1][len(groups[i-1])-1]
		curFirst := groups[i][0]
		if prevLast != curFirst {
			t.Fatalf("packet %d should overlap: prev=%v cur=%v", i, prevLast, curFirst)
		}
	}
}

func TestPermutationsCount(t *testing.T) {
	L := []int{1, 2, 3}
	count := 0
	for range Permutations(L, 2) {
		count++
	}
	if count != 6 {
		t.Fatalf("P(3,2)=6, got %d", count)
	}
}

func TestMaxMin(t *testing.T) {
	if Max(1, 2) != 2 || Min(1, 2) != 1 {
		t.Fatal("max/min")
	}
	if math.Abs(float64(Max(1.5, 2.5)-2.5)) > 1e-9 {
		t.Fatal("float max")
	}
}
