package tsc

import (
	"fmt"
	"github.com/templexxx/cpu"
	"math"
	"testing"
	"time"
)

// Out-of-Order test, getInOrder should be in order as we assume.
func TestGetInOrder(t *testing.T) {

	if !Enabled {
		t.Skip("tsc is disabled")
	}

	n := 4096
	ret0 := make([]uint64, n)
	ret1 := make([]uint64, n)

	for i := range ret0 {
		ret0[i] = getInOrder()
		ret1[i] = getInOrder()
	}

	cnt := 0
	for i := 0; i < n; i++ {
		d := ret1[i] - ret0[i]
		if d < 0 {
			cnt++
		}
	}
	if cnt > 0 {
		t.Fatal(fmt.Sprintf("getInOrder is not in order: %d aren't in order", cnt))
	}
}

func TestUnixNanoCmpWallClock(t *testing.T) {

	if !Enabled {
		t.Skip("tsc is disabled")
	}

	n := 1024

	timeline := make([]int64, n+n+1)

	timeline[0] = UnixNano()
	// [un, wc, un, wc, ..., un]
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = time.Now().UnixNano()
		timeline[i+1] = UnixNano()
	}

	// Try to pick up more accurate time in time.Now(),
	// same logic as func calibrate().
	var sum, max float64
	var min = float64(math.MaxInt64)
	cnt := 0
	for i := 1; i < len(timeline)-1; i += 2 {
		last := timeline[i]
		for j := i + 2; j < len(timeline)-1; j += 2 {
			if timeline[j] != last {
				mid := (i + j - 2) >> 1
				if isEven(mid) {
					mid++
				}

				un := (timeline[mid+1] + timeline[mid-1]) >> 1
				delta := math.Abs(float64(un - timeline[mid]))
				sum += delta
				cnt++
				if delta > max {
					max = delta
				}
				if delta < min {
					min = delta
				}
				i = j
				last = timeline[j]
			}
		}
	}

	avg := sum / float64(cnt)

	t.Log(fmt.Sprintf("tries: %d, pick: %d; delta avg: %0.2fns, min: %0.2fns, max: %0.2fns",
		n, cnt, avg, min, max))

	if avg > 10000 {
		t.Fatal("delta avg > 10000ns, clock jitter or tsc broken")
	}
}

func TestUnixNanoCmpUnixNano(t *testing.T) {

	if !Enabled {
		t.Skip("tsc is disabled")
	}

	// UnixNano is extremely fast, so cache miss will impact a lot.
	// 256 * 2 * 8B = 4KB, we could warm timeline up.
	n := 256

	timeline := make([]int64, n+n+1)

	for i := range timeline { // Warm up.
		timeline[i] = 0
	}

	for i := 0; i < len(timeline); i++ {
		timeline[i] = UnixNano()
	}

	var sum, max float64
	var min = float64(math.MaxInt64)
	cnt := 0
	// For UnixNano, we don't need to pick time as what we did in comparing with wall clock.
	for i := 1; i < len(timeline)-1; i += 2 {
		un := (timeline[i+1] + timeline[i-1]) >> 1
		delta := math.Abs(float64(un - timeline[i]))
		sum += delta
		cnt++
		if delta > max {
			max = delta
		}
		if delta < min {
			min = delta
		}
	}

	avg := sum / float64(cnt)

	t.Log(fmt.Sprintf("tries: %d, pick: %d; delta avg: %0.2fns, min: %0.2fns, max: %0.2fns",
		n, cnt, avg, min, max))

	if avg > 500 { // Even with Out-of-Order, the delta should be as large as 500ns.
		t.Fatal("delta avg > 500ns, tsc broken")
	}
}

func TestIsEven(t *testing.T) {
	for i := 0; i < 13; i += 2 {
		if !isEven(i) {
			t.Fatal("should be even")
		}
	}

	for i := 1; i < 13; i += 2 {
		if isEven(i) {
			t.Fatal("should be odd")
		}
	}
}

func BenchmarkGetInOrder(b *testing.B) {

	if !Enabled {
		b.Skip("tsc is disabled")
	}

	for i := 0; i < b.N; i++ {
		_ = getInOrder()
	}
}

func BenchmarkUnixNano(b *testing.B) {

	if !Enabled {
		b.Skip("tsc is disabled")
	}

	for i := 0; i < b.N; i++ {
		//_ = UnixNano()
		_ = time.Now().UnixNano()
	}
}

func TestCPUName(t *testing.T) {
	fmt.Println(cpu.X86.Name)
	fmt.Println(cpu.X86.Signature)
	fmt.Println(cpu.X86.SteppingID)
	fmt.Println(cpu.X86.TSCFrequency)
}

func TestWallClocks(t *testing.T) {
	cs := make([]int64, 100)
	for i := 0; i < 100; i++ {
		cs[i] = time.Now().UnixNano()
	}
	for i := range cs {
		fmt.Println(cs[i])
	}
}

func TestFloat64Uint64(t *testing.T) {
	var freq uint64 = 3000001814
	c := 1 / (float64(freq) / 1e9)
	fmt.Println(1 / c)

	cc := math.Float64bits(c)
	c = math.Float64frombits(cc)
	fmt.Println(c)
}
