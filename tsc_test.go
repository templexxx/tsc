package tsc

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"
)

func init() {

	ForceTSC()
}

// Out-of-Order test, GetInOrder should be in order as we assume.
func TestGetInOrder(t *testing.T) {

	if !Enabled() {
		t.Skip("tsc is disabled")
	}

	n := 4096
	ret0 := make([]int64, n)
	ret1 := make([]int64, n)

	for i := range ret0 {
		ret0[i] = GetInOrder()
		ret1[i] = GetInOrder()
	}

	cnt := 0
	for i := 0; i < n; i++ {
		d := ret1[i] - ret0[i]
		if d < 0 {
			cnt++
		}
	}
	if cnt > 0 {
		t.Fatal(fmt.Sprintf("GetInOrder is not in order: %d aren't in order", cnt))
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

	if !Enabled() {
		b.Skip("tsc is disabled")
	}

	for i := 0; i < b.N; i++ {
		_ = GetInOrder()
	}
}

func BenchmarkRDTSC(b *testing.B) {

	if !Enabled() {
		b.Skip("tsc is disabled")
	}

	for i := 0; i < b.N; i++ {
		_ = RDTSC()
	}
}

func BenchmarkUnixNano(b *testing.B) {

	if !Enabled() {
		b.Skip("tsc is disabled")
	}

	for i := 0; i < b.N; i++ {
		_ = UnixNano()
	}
}

func BenchmarkSysTime(b *testing.B) {

	for i := 0; i < b.N; i++ {
		_ = time.Now().UnixNano()
	}
}

func TestFastCheckDrift(t *testing.T) {

	if !Enabled() {
		t.Skip("tsc is disabled")
	}

	time.Sleep(time.Second)
	tscc := UnixNano()
	wallc := time.Now().UnixNano()

	if math.Abs(float64(tscc-wallc)) > 10000 { // Which means every second may have 20us drift, too much.
		t.Log(tscc - wallc)
		t.Fatal("the tsc frequency is too far away from the real, please use tools/calibrate to find out potential issues")
	}
}

// TestCalibrate with race detection.
func TestCalibrate(t *testing.T) {

	if !Enabled() {
		t.Skip("tsc is disabled")
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func(ctx context.Context) {

		ctx2, cancel2 := context.WithCancel(ctx)
		defer cancel2()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				calibrate(true, true)
			case <-ctx2.Done():
				break
			}
		}
	}(ctx)

	time.Sleep(3 * time.Second)
	cancel()
}
