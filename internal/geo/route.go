package geo

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

func ParameterToString(obj interface{}, delimiter string) string {
	if reflect.TypeOf(obj).Kind() == reflect.Slice || reflect.TypeOf(obj).Kind() == reflect.Array {
		return strings.Trim(strings.Replace(fmt.Sprint(obj), " ", delimiter, -1), "[]")
	} else if t, ok := obj.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	return fmt.Sprintf("%v", obj)
}

func FloatEEToPolyline(floatee [][2]float32, delimiter string) string {
	var points []string
	for _, v := range floatee {
		points = append(points, ParameterToString(v, ","))
	}
	return ParameterToString(points, delimiter)
}

func WaypointsPacket(batch int, waypoints ...[2]float32) [][][2]float32 {
	var groups [][][2]float32
	for i := range Range(0, len(waypoints), batch) {
		start, end := i, i+batch
		if start > 0 {
			start--
		}
		if end > len(waypoints) {
			end = len(waypoints)
		}
		groups = append(groups, waypoints[start:end])
	}
	return groups
}

func Range[T int](args ...T) chan T {
	if l := len(args); l < 1 || l > 3 {
		fmt.Println("error args length, Range requires 1-3 int arguments")
	}
	var start, stop T
	var step T = 1
	switch len(args) {
	case 1:
		stop = args[0]
		start = 0
	case 2:
		start, stop = args[0], args[1]
	case 3:
		start, stop, step = args[0], args[1], args[2]
	}

	ch := make(chan T)
	go func() {
		if step > 0 {
			for start < stop {
				ch <- start
				start = start + step
			}
		} else {
			for start > stop {
				ch <- start
				start = start + step
			}
		}
		close(ch)
	}()
	return ch
}

func In[T int | int8 | int16 | int32 | int64 | float32 | float64 | string](target T, array []T) bool {
	for _, element := range array {
		if target == element {
			return true
		}
	}
	return false
}

func Permutations[T any](L []T, r int) chan []T {
	ch := make(chan []T)
	go func() {
		if r == 1 {
			for _, x := range L {
				ch <- []T{x}
			}
		} else {
			for x := range Range(0, len(L)) {
				newL := append([]T{}, L[:x]...)
				newL = append(newL, L[x+1:]...)
				for y := range Permutations(newL, r-1) {
					res := append([]T{}, L[x])
					res = append(res, y...)
					ch <- res
				}
			}
		}
		close(ch)
	}()
	return ch
}
