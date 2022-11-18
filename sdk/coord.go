// Package sdk
// @Author Euraxluo  11:02:00
package sdk

import (
	"github.com/the-go-tool/cast"
	"math"
	"regexp"
	"strings"
)

const (
	XPi         = 52.35987755982988      // Pi*3000/180
	Axis        = 6378245.0              // 长半轴
	Offset      = 0.00669342162296594323 // 偏心率平方
	BDLonOffset = 0.0065                 // 百度坐标系经度偏移常量
	BDLatOffset = 0.0060                 // 百度坐标系纬度偏移常量
)

type U interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// Max returns the larger one of a and b.
func Max[T U](a, b T) T {
	if a > b {
		return a
	}

	return b
}

// Min returns the smaller one of a and b.
func Min[T U](a, b T) T {
	if a < b {
		return a
	}

	return b
}

// 计算偏移量
func delta[T U](ilon, ilat T) (float32, float32) {
	lon := float64(ilon)
	lat := float64(ilat)

	dlat, dlon := transform(lon-105.0, lat-35.0)
	radlat := lat / 180.0 * math.Pi
	magic := math.Sin(radlat)
	magic = 1 - Offset*magic*magic
	sqrtmagic := math.Sqrt(magic)
	dlat = (dlat * 180.0) / ((Axis * (1 - Offset)) / (magic * sqrtmagic) * math.Pi)
	dlon = (dlon * 180.0) / (Axis / sqrtmagic * math.Cos(radlat) * math.Pi)
	return float32(lon + dlon), float32(lat + dlat)
}

// 转换逻辑
func transform(lon, lat float64) (float64, float64) {
	var lonlat = lon * lat
	var absX = math.Sqrt(math.Abs(lon))
	var lonPi, latPi = lon * math.Pi, lat * math.Pi
	var d = 20.0*math.Sin(6.0*lonPi) + 20.0*math.Sin(2.0*lonPi)
	x, y := d, d
	x += 20.0*math.Sin(latPi) + 40.0*math.Sin(latPi/3.0)
	y += 20.0*math.Sin(lonPi) + 40.0*math.Sin(lonPi/3.0)
	x += 160.0*math.Sin(latPi/12.0) + 320*math.Sin(latPi/30.0)
	y += 150.0*math.Sin(lonPi/12.0) + 300.0*math.Sin(lonPi/30.0)
	x *= 2.0 / 3.0
	y *= 2.0 / 3.0
	x += 2.0*lon + 3.0*lat + 0.2*lat*lat + 0.1*lonlat + 0.2*absX - 100.0
	y += lon + 2.0*lat + 0.1*lon*lon + 0.1*lonlat + 0.1*absX + 300.0
	return x, y
}

// InChina 中国范围：lon: 73.66~135.05  lat: 3.86~53.55
func InChina[T U](lon, lat T) bool {
	return (73.66 < float32(lon) && float32(lon) < 135.05) && (3.86 < float32(lat) && float32(lat) < 53.55)
}

//WGS84toGCJ02 WGS84坐标系->火星坐标系
func WGS84toGCJ02[T U](lon, lat T) (float32, float32) {
	if !InChina(lon, lat) {
		return float32(lon), float32(lat)
	}
	return delta(lon, lat)
}

//GCJ02toBD09 火星坐标系->百度坐标系
func GCJ02toBD09[T U](ilon, ilat T) (float32, float32) {
	lon := float64(ilon)
	lat := float64(ilat)
	z := math.Sqrt(lon*lon+lat*lat) + 0.00002*math.Sin(lat*XPi)
	theta := math.Atan2(lat, lon) + 0.000003*math.Cos(lon*XPi)
	return float32(z*math.Cos(theta) + BDLonOffset), float32(z*math.Sin(theta) + BDLatOffset)
}

//BD09toGCJ02 百度坐标系->火星坐标系
func BD09toGCJ02[T U](lon, lat T) (float32, float32) {
	var x = float64(lon) - BDLonOffset
	var y = float64(lat) - BDLatOffset
	z := math.Sqrt(x*x+y*y) - 0.00002*math.Sin(y*XPi)
	theta := math.Atan2(y, x) - 0.000003*math.Cos(x*XPi)
	return float32(z * math.Cos(theta)), float32(z * math.Sin(theta))
}

//GCJ02toWGS84 火星坐标系->WGS84坐标系(微调)
func GCJ02toWGS84[T U](ilon, ilat T) (float32, float32) {
	lon := float64(ilon)
	lat := float64(ilat)
	threshold := 0.0000000001 // 设置精准率阙值
	mlon := lon - 0.01
	mlat := lat - 0.01
	plon := lon + 0.01
	plat := lat + 0.01
	var dlon, dlat, wgsLat, wgsLon float64
	for i := 0; i < 10000; i++ {
		wgsLat = (mlat + plat) / 2
		wgsLon = (mlon + plon) / 2
		tmpLon, tmpLat := delta(wgsLon, wgsLat)
		dlon = float64(tmpLon) - lon
		dlat = float64(tmpLat) - lat
		if math.Abs(dlat) < threshold && math.Abs(dlon) < threshold {
			break
		}
		if dlat > 0 {
			plat = wgsLat
		} else {
			mlat = wgsLat
		}

		if dlon > 0 {
			plon = wgsLon
		} else {
			mlon = wgsLon
		}
	}
	return float32(wgsLon), float32(wgsLat)
}

// Distance 计算
func Distance[T U](lng1, lat1, lng2, lat2 T) float32 {
	latn1 := cast.To[float64](lat1)
	lngn1 := cast.To[float64](lng1)
	latn2 := cast.To[float64](lat2)
	lngn2 := cast.To[float64](lng2)

	radius := 6371000.0 //6378137.0
	rad := math.Pi / 180.0
	latn1 = latn1 * rad
	lngn1 = lngn1 * rad
	latn2 = latn2 * rad
	lngn2 = lngn2 * rad
	theta := lngn2 - lngn1
	dist := math.Acos(math.Sin(latn1)*math.Sin(latn2) + math.Cos(latn1)*math.Cos(latn2)*math.Cos(theta))
	return float32(dist * radius)
}
func CoordListCast[F U, R U](coords [][]float32, CastFunc func(lon, lat F) (float32, float32)) [][2]R {
	var result [][2]R
	for _, v := range coords {
		x, y := CastFunc(cast.To[F](v[0]), cast.To[F](v[1]))
		result = append(result, [2]R{
			cast.To[R](x),
			cast.To[R](y),
		})
	}
	return result
}

func CoordCastList[F U, R U](coords [][2]float32, CastFunc func(lon, lat F) (float32, float32)) [][]R {
	var result [][]R
	for _, v := range coords {
		x, y := CastFunc(cast.To[F](v[0]), cast.To[F](v[1]))
		result = append(result, []R{
			cast.To[R](x),
			cast.To[R](y),
		})
	}
	return result
}

var (
	jsonCheck = regexp.MustCompile(`(?i:(?:application|text)/(?:vnd\.[^;]+\+)?json)`)
	xmlCheck  = regexp.MustCompile(`(?i:(?:application|text)/xml)`)
)

// FloatEEToPolyline  将FloatEE转换为字符串形式的
func FloatEEToPolyline(floatee [][2]float32, delimiter string) (polyline string) {
	var points []string
	for _, v := range floatee {
		points = append(points, ParameterToString(v, ","))
	}
	return ParameterToString(points, delimiter)
}

// PolylineToFloatEE  将字符串形式的polyline 转换为FloatEE
func PolylineToFloatEE(polyline string, delimiter string) (result [][2]float32) {
	for _, location := range strings.Split(polyline, delimiter) {
		var pointArray [2]float32
		for j, p := range strings.Split(location, ",") {
			pointArray[j] = cast.To[float32](p)
		}
		result = append(result, pointArray)
	}
	return
}
