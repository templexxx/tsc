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
	// Using false sharing range as aligned size & total size for avoiding cache pollution.
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
// 	start := tsc.UnixNano()
// 	foo()
// 	end := tsc.UnixNano()
// 	cost := end - start
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

var (
	linuxClockSourcePath = "/sys/devices/system/clocksource/clocksource0/current_clocksource"
)

// GetCurrentClockSource gets clock source on Linux.
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
