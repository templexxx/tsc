package tsc

import "time"

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
// See getInOrder in tsc_amd64.s for more details.
func UnixNano() int64 {
	return unixNano()
}

var unixNano = func() int64 {
	return time.Now().UnixNano()
}

// Enabled indicates tsc could work or not.
// If true, use tsc time. Otherwise, use time.Now().
var Enabled = false

func isEven(n int) bool {
	return n&1 == 0
}
