package tsc

import (
	"sync/atomic"
	"time"
)

// UnixNano returns t as a Unix time, the number of nanoseconds elapsed
// since January 1, 1970 UTC.
//
// Warn:
// UnixNano is not designed for benchmarking purpose,
// DO NOT use it for measuring function performance.
//
// e.g.
// ```
// 	start := tsc.UnixNano()
// 	foo()
// 	end := tsc.UnixNano()
// 	cost := end - start
// ```
// The value of cost is unpredictable,
// because all instructions for getting tsc are not serializing,
// we need to be careful to deal with the order (use barrier).
//
// Although I have implemented a tsc getting method with order,
// Go is not designed for HPC, and it's own benchmark testing is
// enough in most cases. I think I should go further to explore to
// make sure it's best practice for counting cycles.
// See GetInOrder in tsc_amd64.s for more details.
func UnixNano() int64 {
	return unixNano()
}

var unixNano = func() int64 {
	return time.Now().UnixNano()
}

var (
	enabled int64 = 0
)

// Enabled indicates tsc could work or not(using TSC register as clock source).
// If true, use tsc time. Otherwise, use time.Now().
func Enabled() bool {
	return atomic.LoadInt64(&enabled) == 1
}

var (
	supported int64 = 0
)

// Supported indicates Invariant TSC supported.
// But may still be !Enabled because lacking of stable tsc frequency.
func Supported() bool {
	return atomic.LoadInt64(&supported) == 1
}

var (
	allowUnstableFreq int64 = 0
)

// AllowUnstableFreq allows to get tsc frequency at starting in a fast way(without long run testing),
// it's useful for an application which doesn't need accurate clock but just want the speed of getting
// timestamp could be faster.
func AllowUnstableFreq() bool {
	return atomic.LoadInt64(&allowUnstableFreq) == 1
}

// ResetEnabled tries to reset Enabled by passing allow UnstableFreq.
// Return true, if Enabled.
func ResetEnabled(allow bool) bool {
	if allow == false {
		atomic.StoreInt64(&allowUnstableFreq, 0)
	} else {
		atomic.StoreInt64(&allowUnstableFreq, 1)
	}
	return reset()
}

// FreqSource is the source of tsc frequency.
// Empty means no available source, tsc is !Enabled.
var FreqSource = ""

const (
	// EnvSource means this lib gets tsc frequency from environment variable.
	EnvSource = "env"
	// CPUFeatureSource means this lib gets tsc frequency from https://github.com/templexxx/cpu
	CPUFeatureSource = "cpu_feat"
	// FastDetectSource means this lib get tsc frequency from a fast detection.
	FastDetectSource = "fast_detect"
)

func isEven(n int) bool {
	return n&1 == 0
}
