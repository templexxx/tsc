package tsc

import (
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/templexxx/cpu"
	"github.com/templexxx/tsc/internal/xbytes"
)

var (
	supported int64 = 0 // Whether Invariant TSC is supported.
	// Set to 1 by calling AllowOutOfOrder() when out-of-order execution is acceptable.
	// For example, logging can usually tolerate nanosecond-level timestamp reordering.
	allowOutOfOrder int64 = 1
)

// unix_nano_timestamp = tsc_register_value * Coeff + Offset.
// Coeff = 1 / (tsc_frequency / 1e9).
// coeff can be treated as the inverse of TSC frequency in GHz for conversion.
// Multiplication is much faster than division on this hot path.
var (
	// OffsetCoeff is offset & coefficient pair.
	// Coefficient is in [0,64) bits.
	// Offset is in [64, 128) bits.
	// Use false-sharing range as alignment and block size to reduce cache-line contention.
	OffsetCoeff     = xbytes.MakeAlignedBlock(cpu.X86FalseSharingRange, cpu.X86FalseSharingRange)
	OffsetCoeffAddr = &OffsetCoeff[0]
)

var (
	// OffsetCoeffF using float64 as offset.
	OffsetCoeffF     = xbytes.MakeAlignedBlock(cpu.X86FalseSharingRange, cpu.X86FalseSharingRange)
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

// Supported reports whether Invariant TSC is supported.
func Supported() bool {
	return supported == 1
}

// AllowOutOfOrder sets allowOutOfOrder true.
//
// Not thread-safe.
func AllowOutOfOrder() {

	if !Supported() {
		return
	}

	allowOutOfOrder = 1

	reset()
}

// ForbidOutOfOrder sets allowOutOfOrder false.
//
// Not thread-safe.
func ForbidOutOfOrder() {

	if !Supported() {
		return
	}

	allowOutOfOrder = 0

	reset()
}

// IsOutOfOrder reports whether out-of-order execution is allowed.
//
// Not thread-safe.
func IsOutOfOrder() bool {
	return allowOutOfOrder == 1
}

func isEven(n int) bool {
	return n&1 == 0
}

var (
	linuxClockSourcePath = "/sys/devices/system/clocksource/clocksource0/current_clocksource"
)

// GetCurrentClockSource returns the Linux kernel clock source.
// Returns an empty string on non-Linux platforms or read errors.
func GetCurrentClockSource() string {

	if runtime.GOOS != "linux" {
		return ""
	}

	f, err := os.Open(linuxClockSourcePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	d, err := ioutil.ReadAll(f)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(d), "\n")
}
