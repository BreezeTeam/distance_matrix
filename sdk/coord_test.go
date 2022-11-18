// Package sdk
// @Author Euraxluo  11:21:00
package sdk

import "testing"

func TestDistance(t *testing.T) {
	type args struct {
		lat1 float64
		lng1 float64
		lat2 float64
		lng2 float64
	}
	tests := []struct {
		name string
		args args
	}{
		{"test1", args{23.1378010917, 113.4022203113, 22.1191433172, 113.5826193044}},
		{"test2", args{39.923423, 116.368904, 39.922501, 116.387271}}, //高德经纬度,官方距离为 1571.3795380
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Distance(tt.args.lng1, tt.args.lat1, tt.args.lng2, tt.args.lat2)
			t.Log(float32(got))
		})
	}
}
