// Package sdk
// @Author Euraxluo  17:56:00
package sdk

import (
	"fmt"
	"testing"
)

func Test_pointsPermutationsSorted(t *testing.T) {
	res := pointsPermutationsSorted([][2]float32{{1, 2}, {4, 3}, {4, 6}})
	fmt.Printf("%v \n", res)
}
