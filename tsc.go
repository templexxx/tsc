package tsc

import "time"

var (
	supported int64 = 0 // Supported invariant TSC or not.
	stable    int64 = 0 // TSC frequency is stable or not. (If not, we may have big gap between wall clock after long run)
	forceTSC  int64 = 0 // Enable TSC no matter it's stable or not.
	enabled   int64 = 0 // TSC clock source is enabled or not, if yes, getting timestamp by tsc register.
	// Set it to 1 by invoke AllowOutOfOrder() if out-of-order execution is acceptable.
	// e.g., for logging, backwards is okay in nanoseconds level.
	allowOutOfOrder int64 = 1
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
// Go is not designed for HPC, and its own benchmark testing is
// enough in most cases. I think I should go further to explore to
// make sure it's best practice for counting cycles.
// See GetInOrder in tsc_amd64.s for more details.
var UnixNano func() int64

func sysClock() int64 {
	return time.Now().UnixNano()
}

// Enabled indicates TSC clock source is enabled or not (using TSC register as clock source).
// If true, use TSC time. Otherwise, use time.Now().
func Enabled() bool {
	return enabled == 1
}

// Supported indicates Invariant TSC supported.
func Supported() bool {
	return supported == 1
}

// Stable indicates TSC frequency is stable or not. (If not, we may have big gap between wall clock after long run)
func Stable() bool {
	return stable == 1
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

// ForceTSC forces using TSC as clock source.
// Returns false if TSC is unsupported.
//
// Warn:
// Not thread safe, using it at application starting.
func ForceTSC() bool {

	if Enabled() {
		return true
	}

	if !Supported() {
		return false
	}

	forceTSC = 1

	return reset()
}

func isEven(n int) bool {
	return n&1 == 0
}
