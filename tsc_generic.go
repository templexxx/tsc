// +build !amd64

package tsc

func init() {
	UnixNano = sysClock
}

// Calibrate calibrates tsc & wall clock.
//
// It's a good practice that run Calibrate period (every hour) outside,
// because the wall clock may be calibrated (e.g. NTP).
//
// If !enabled do nothing.
func Calibrate() {

	return
}

// GetInOrder gets tsc value in strictly order.
// It's used for helping calibrate to avoid out-of-order issues.
//
// For non-amd64, just return 0.
func GetInOrder() uint64 {
	return 0
}

func reset() bool {
	return false
}

// GetInOrder gets tsc value in strictly order.
// It's used for helping calibrate to avoid out-of-order issues.
// For non-amd64, just return 0.
func GetInOrder() uint64 {
	return 0
}

// SetFreq sets frequency manually.
func SetFreq(freq float64) {
	return
}
