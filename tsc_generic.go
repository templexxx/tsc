// +build !amd64

package tsc

// Calibrate calibrates tsc & wall clock.
//
// It's a good practice that run Calibrate period (every hour) outside,
// because the wall clock may be calibrated (e.g. NTP).
//
// If !enabled do nothing.
func Calibrate() {

	return
}

// getInOrder gets tsc value in strictly order.
// It's used for helping calibrate to avoid out-of-order issues.
func getInOrder() uint64 {
	return 0
}
