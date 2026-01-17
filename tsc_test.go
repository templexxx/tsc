package tsc

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestIsEven(t *testing.T) {
	t.Parallel()

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

	for range b.N {
		_ = UnixNano()
	}
}

func BenchmarkSysTime(b *testing.B) {
	for range b.N {
		_ = time.Now().UnixNano()
	}
}

func TestFastCheckDrift(t *testing.T) {
	t.Parallel()

	if !Supported() {
		t.Skip("tsc is unsupported")
	}

	// Skip when race detector is enabled - it adds unpredictable timing overhead
	// that corrupts calibration and drift measurements
	if raceDetectorEnabled {
		t.Skip("race detector affects timing accuracy")
	}

	// Perform fresh calibration to ensure accuracy
	Calibrate()

	// Measure drift immediately after calibration
	// Multiple measurements to account for OS timing jitter
	drifts := make([]int64, 10)
	for i := range drifts {
		tscc := UnixNano()
		wallc := time.Now().UnixNano()
		drifts[i] = tscc - wallc
	}

	// Use average to filter out outliers
	var totalDrift int64
	for _, d := range drifts {
		totalDrift += d
	}

	avgDrift := totalDrift / int64(len(drifts))

	// 50us threshold accounts for OS timing precision limitations
	// (time.Now() has ~1us precision on macOS/Windows vs nanosecond on Linux)
	if math.Abs(float64(avgDrift)) > 50000 {
		t.Logf("average drift: %d ns (%.2f us)", avgDrift, float64(avgDrift)/1000)
		t.Fatal("the tsc frequency is too far away from the real, please use tools/calibrate to find out potential issues")
	}
}

// TestCalibrate with race detection.
//
//nolint:paralleltest
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
