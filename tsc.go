package tsc

import (
	"io"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/templexxx/tsc/internal/xbytes"
)

const (
	// CacheLineSize is the cache line size used for alignment (64 bytes for both x86 and ARM64).
	CacheLineSize = 64
)

var (
	supported int64 = 0 // Supported invariant TSC or not.
	// Set it to 1 by invoke AllowOutOfOrder() if out-of-order execution is acceptable.
	// e.g., for logging, backwards is okay in nanoseconds level.
	allowOutOfOrder int64 = 1
)

// unix_nano_timestamp = tsc_register_value * Coeff + Offset.
// Coeff = 1 / (tsc_frequency / 1e9).
// We could regard coeff as the inverse of TSCFrequency(GHz) (actually it just has mathematics property)
// for avoiding future dividing.
// MUL gets much better performance than DIV.
var (
	// OffsetCoeff is offset & coefficient pair.
	// Coefficient is in [0,64) bits.
	// Offset is in [64, 128) bits.
	// Using cache line size for alignment to avoid cache pollution.
	OffsetCoeff     = xbytes.MakeAlignedBlock(CacheLineSize, CacheLineSize)
	OffsetCoeffAddr = &OffsetCoeff[0]
)

var (
	// OffsetCoeffF using float64 as offset.
	OffsetCoeffF     = xbytes.MakeAlignedBlock(CacheLineSize, CacheLineSize)
	OffsetCoeffFAddr = &OffsetCoeffF[0]
)

// UnixNano returns time as a Unix time, the number of nanoseconds elapsed
// since January 1, 1970 UTC.
//
// Warn:
// DO NOT use it for measuring single function performance unless ForbidOutOfOrder has been invoked.
//
// e.g.
// ```
//
//	start := tsc.UnixNano()
//	foo()
//	end := tsc.UnixNano()
//	cost := end - start
//
// ```
// The value of cost is unpredictable,
// because all instructions for getting tsc are not serializing,
// we need to be careful to deal with the order (use barrier).
//
// See GetInOrder in tsc_amd64.s for more details.
var UnixNano = sysClock

func sysClock() int64 {
	return time.Now().UnixNano()
}

// Supported indicates Invariant TSC supported.
func Supported() bool {
	return supported == 1
}

// AllowOutOfOrder sets allowOutOfOrder true.
//
// Not threads safe.
func AllowOutOfOrder() {
	if !Supported() {
		return
	}

	allowOutOfOrder = 1

	reset()
}

// ForbidOutOfOrder sets allowOutOfOrder false.
//
// Not threads safe.
func ForbidOutOfOrder() {
	if !Supported() {
		return
	}

	allowOutOfOrder = 0

	reset()
}

// IsOutOfOrder returns allow out-of-order or not.
//
// Not threads safe.
func IsOutOfOrder() bool {
	return allowOutOfOrder == 1
}

func isEven(n int) bool {
	return n&1 == 0
}

// GetCurrentClockSource gets clock source on Linux.
func GetCurrentClockSource() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	const linuxClockSourcePath = "/sys/devices/system/clocksource/clocksource0/current_clocksource"

	file, err := os.Open(linuxClockSourcePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	d, err := io.ReadAll(file)
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(d), "\n")
}

// simpleLinearRegression performs simple linear regression on counter vs system time.
// Shared by both AMD64 and ARM64 calibration.
func simpleLinearRegression(tscs, syss []float64) (float64, int64) {
	tmean, wmean := float64(0), float64(0)
	for _, i := range tscs {
		tmean += i
	}

	for _, i := range syss {
		wmean += i
	}

	tmean /= float64(len(tscs))
	wmean /= float64(len(syss))

	denominator, numerator := float64(0), float64(0)
	for i := range tscs {
		numerator += (tscs[i] - tmean) * (syss[i] - wmean)
		denominator += (tscs[i] - tmean) * (tscs[i] - tmean)
	}

	coeff := numerator / denominator

	return coeff, int64(wmean - coeff*tmean)
}

// getClosestTSCSys tries to get the closest counter value nearby the system clock in a loop.
// Shared by both AMD64 and ARM64 calibration.
func getClosestTSCSys(n int) (int64, int64) {
	// 256 is enough for finding the lowest sys clock cost in most cases.
	// Although time.Now() is using VDSO to get time, but it's unstable,
	// sometimes it will take more than 1000ns,
	// we have to use a big loop(e.g. 256) to get the "real" clock.
	// And it won't take a long time to finish a calibrating job, only about 20µs.
	// [tscClock, wc, tscClock, wc, ..., tscClock]
	timeline := make([]int64, n+n+1)

	timeline[0] = RDTSC()
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = time.Now().UnixNano()
		timeline[i+1] = RDTSC()
	}

	// The minDelta is the smallest gap between two adjacent counter readings,
	// which means the smallest gap between sys clock and counter too.
	minDelta := int64(math.MaxInt64)
	minIndex := 1 // minIndex is sys clock index where has minDelta.

	// time.Now()'s precision is only µs (on macOS),
	// which means we will get the multi-same sys clock in timeline,
	// and the middle one is closer to the real time in statistics.
	// Try to find the minimum delta when sys clock is in the "middle".
	for i := 1; i < len(timeline)-1; i += 2 {
		last := timeline[i]
		for j := i + 2; j < len(timeline)-1; j += 2 {
			if timeline[j] != last {
				mid := (i + j - 2) >> 1
				if isEven(mid) {
					mid++
				}

				delta := timeline[mid+1] - timeline[mid-1]
				if delta < minDelta {
					minDelta = delta
					minIndex = mid
				}

				i = j
				last = timeline[j]
			}
		}
	}

	tscClock := (timeline[minIndex+1] + timeline[minIndex-1]) >> 1
	sys := timeline[minIndex]

	return tscClock, sys
}
