// Package sdk
// @Author Euraxluo  10:38:00
package sdk

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/the-go-tool/cast"
	"github.com/the-go-tool/cast/core/casters"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strings"
	"time"
)

func init() {
	cast.MustRegister(func(in json.Number) (out string, err error) {
		return in.String(), nil
	})
	cast.MustRegisterProxy[json.Number, int, string]()
	cast.MustRegisterProxy[json.Number, int8, string]()
	cast.MustRegisterProxy[json.Number, int16, string]()
	cast.MustRegisterProxy[json.Number, int32, string]()
	cast.MustRegisterProxy[json.Number, int64, string]()
	cast.MustRegisterProxy[json.Number, float32, string]()
	cast.MustRegisterProxy[json.Number, float64, string]()
}

type RequestFunc func(req *http.Request) (*http.Response, error)

// LogRequest  进行请求，并打日志
func LogRequest(path string, queryParams url.Values, body io.Reader, debug bool, f RequestFunc, method string) func() (*http.Response, error) {
	return func() (*http.Response, error) {
		//  解析url，转为url对象
		requestUrl, err := url.Parse(path)
		if err != nil {
			return nil, err
		}
		query := requestUrl.Query()
		for k, v := range queryParams {
			for _, iv := range v {
				query.Add(k, iv)
			}
		}
		requestUrl.RawQuery = query.Encode()
		log.Printf("%s\n", requestUrl.String())
		request, err := http.NewRequest(method, requestUrl.String(), body)
		if err != nil {
			return nil, err
		}
		if debug {
			dump, err := httputil.DumpRequestOut(request, true)
			if err != nil {
				return nil, err
			}
			log.Printf("%s\n", string(dump)) //ZERO-LOG
		} else {
			return f(request)
		}
		if resp, err := f(request); err != nil {
			log.Printf("%s\n", err.Error())
			return resp, err
		} else {
			dump, err := httputil.DumpResponse(resp, true)
			if err != nil {
				return resp, err
			}
			log.Printf("%s\n", string(dump))
			return resp, err
		}
	}
}

// ParameterToString convert interface{} parameters to string, using a delimiter if format is provided. TODO:TEST
func ParameterToString(obj interface{}, delimiter string) string {
	if reflect.TypeOf(obj).Kind() == reflect.Slice || reflect.TypeOf(obj).Kind() == reflect.Array {
		return strings.Trim(strings.Replace(fmt.Sprint(obj), " ", delimiter, -1), "[]")
	} else if t, ok := obj.(time.Time); ok {
		return t.Format(time.RFC3339)
	}

	return fmt.Sprintf("%v", obj)
}

// StructToMap  结构体转为Map[string]interface{}
func StructToMap(in interface{}, tagName string) map[string]interface{} {
	out := make(map[string]interface{})
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct { // 非结构体返回错误提示
		panic("StructToMap input must struct")
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fi := t.Field(i)
		if tagName == "" {
			out[fi.Name] = v.Field(i).Interface()
		} else if tagValue := fi.Tag.Get(tagName); tagValue != "" {
			out[tagValue] = v.Field(i).Interface()
		}
	}
	return out
}

// MapToStruct  Map[string]interface{}转为结构体
func MapToStruct(m map[string]interface{}, u interface{}, tagName string) {
	v := reflect.ValueOf(u)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic("MapToStruct input must struct")
	}
	findFromMap := func(key string, nameTag string) interface{} {
		for k, v := range m {
			if k == key || k == nameTag {
				return v
			}
		}
		return nil
	}
	for i := 0; i < v.NumField(); i++ {
		val := findFromMap(v.Type().Field(i).Name, v.Type().Field(i).Tag.Get(tagName))
		if val != nil && reflect.ValueOf(val).Kind() == v.Field(i).Kind() {
			v.Field(i).Set(reflect.ValueOf(val).Convert(v.Field(i).Type()))
		} else if reflect.ValueOf(val).Kind() == reflect.String {
			outValue, _ := casters.Get(reflect.ValueOf(val).Type(), v.Field(i).Type())(reflect.ValueOf(val))
			v.Field(i).Set(outValue)

		}
	}
}

// DecodeResponse  输入结构体和http响应体，能够通过xml和json 将数据解析到结构体中
func DecodeResponse(v interface{}, response *http.Response) (err error) {
	var (
		body        []byte
		contentType string
	)
	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	contentType = response.Header.Get("Content-Type")
	_ = response.Body.Close()

	if len(body) == 0 {
		return nil
	}
	if s, ok := v.(*string); ok {
		*s = string(body)
		return nil
	}
	if xmlCheck.MatchString(contentType) {
		if err = xml.Unmarshal(body, v); err != nil {
			return err
		}
		return nil
	}
	if jsonCheck.MatchString(contentType) {
		if err = json.Unmarshal(body, v); err != nil {
			return err
		}
		return nil
	}
	return errors.New("undefined response type")
}

// WaypointsPacket 按照batch 将 Waypoints 切分为多组,并保证每一段路径都在组内
func WaypointsPacket(batch int, waypoints ...[2]float32) (groups [][][2]float32) {
	for i := range Range(0, len(waypoints), batch) {
		var start, end int
		end = i + batch
		start = i
		if start > 0 {
			start = start - 1
		}
		if end > len(waypoints) {
			end = len(waypoints)
		}
		groups = append(groups, waypoints[start:end])
	}
	return
}

// Range  python 的 range
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

type Pair[T, U any] struct {
	First  T
	Second U
}

// Zip  python 的 zip
func Zip[T, U any](ts []T, us []U) []Pair[T, U] {
	pairs := make([]Pair[T, U], Min[int](len(ts), len(us)))
	for i := 0; i < len(pairs); i++ {
		pairs[i] = Pair[T, U]{ts[i], us[i]}
	}
	return pairs
}

// In  判断target 是否在 array中
func In[T int | int8 | int16 | int32 | int64 | float32 | float64 | string](target T, array []T) bool {
	for _, element := range array {
		if target == element {
			return true
		}
	}
	return false
}

// Permutations  python 的Permutations 算法
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
