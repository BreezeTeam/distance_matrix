package persist

import (
	"encoding/json"
	"fmt"
)

func coordJSON(p [2]float32) string {
	b, _ := json.Marshal([]float64{round6(p[0]), round6(p[1])})
	return string(b)
}

func parseCoord(raw string) ([2]float32, error) {
	var v []float64
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return [2]float32{}, err
	}
	if len(v) < 2 {
		return [2]float32{}, fmt.Errorf("bad coord %q", raw)
	}
	return [2]float32{float32(v[0]), float32(v[1])}, nil
}

func round6(x float32) float64 {
	return float64(int64(float64(x)*1e6+0.5)) / 1e6
}
