package tsc

import (
	"context"
	"math"
	"testing"
	"time"
)

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

func BenchmarkUnixNano(b *testing.B) {

	if !Supported() {
		b.Skip("tsc is unsupported")
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

	if !Supported() {
		t.Skip("tsc is unsupported")
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

	if !Supported() {
		t.Skip("tsc is unsupported")
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
				Calibrate()
			case <-ctx2.Done():
				break
			}
		}
	}(ctx)

	time.Sleep(3 * time.Second)
	cancel()
}
