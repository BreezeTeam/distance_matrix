// Package sdk
// @Author Euraxluo  22:12:00
package sdk

import (
	"fmt"
	"testing"
)

func TestBeSnowflakeID(t *testing.T) {
	var total = 0
	for true {
		go func() { println(SnowflakeID()) }()
		if total > 100 {
			break
		}
		total += 1
	}
}

func TestZip(t *testing.T) {
	ts := []uint64{100, 200}
	us := []string{"aa", "bb", "cc"}

	p := Zip(ts, us)
	fmt.Println(p)
}

func TestPermutations(t *testing.T) {
	for x := range Permutations([]int{1, 2, 3, 4}, 4) {
		fmt.Printf("%v\n", x)
	}
}
