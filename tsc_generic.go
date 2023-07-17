//go:build !amd64
// +build !amd64

package tsc

func reset() bool { return false }

// Calibrate calibrates tsc & wall clock.
//
// It's a good practice that run Calibrate period (every hour) outside,
// because the wall clock may be calibrated (e.g. NTP).
//
// If !enabled do nothing.
func Calibrate() {

	return
}

func CalibrateWithCoeff(coeff float64) {
	return
}

// GetInOrder gets tsc value in strictly order.
// It's used for helping calibrate to avoid out-of-order issues.
//
// For non-amd64, just return 0.
func GetInOrder() uint64 {
	return 0
}

func RDTSC() int64 {
	return 0
}

func LoadOffsetCoeff(src *byte) (offset int64, coeff float64) {
	return 0, 0
}
