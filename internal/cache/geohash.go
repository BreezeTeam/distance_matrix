package cache

import (
	"github.com/mmcloughlin/geohash"
)

const defaultPrecision = 8

// EncodeGeoHash encodes lon/lat at precision 8 (Python default).
func EncodeGeoHash(lon, lat float32) string {
	return geohash.EncodeWithPrecision(float64(lat), float64(lon), defaultPrecision)
}
