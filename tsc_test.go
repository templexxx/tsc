package tsc

import (
	"fmt"
	"sort"
	"testing"
)

func TestRDTSCP(t *testing.T) {

	ret := make([]int, 1024)

	for i := range ret {
		a := rdtscp()
		b := rdtscp()
		d := b - a
		ret[i] = int(d) // It's safe for most cases.
	}

	sort.Ints(ret)

	// Drop peaks.
	ret = ret[128:]
	ret = ret[:len(ret)-128]

	sum := 0
	for _, r := range ret {
		sum += r
	}
	avg := sum / len(ret)

	fmt.Println(avg)

	// We want < 10% jitter.
	if ret[len(ret)-1]-avg > 5 {
		t.Fatal("RDTSCP is not reliable as we thought")
	}

	if avg-ret[0] > 5 {
		t.Fatal("RDTSCP is not reliable as we thought")
	}
}

func BenchmarkRDTSCP(b *testing.B) {

	for i := 0; i < b.N; i++ {
		rdtscp()
	}
}
