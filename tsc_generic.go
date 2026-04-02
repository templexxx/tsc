//go:build !amd64
// +build !amd64

package tsc

func reset() bool { return false }

// Calibrate is a no-op on non-amd64 platforms.
//
// Keeping this symbol preserves the package API across architectures.
func Calibrate() {

	return
}

func CalibrateWithCoeff(coeff float64) {
	return
}

// GetInOrder gets TSC in strict order.
// Non-amd64 builds do not support TSC and return zero.
func GetInOrder() uint64 {
	return 0
}

func RDTSC() int64 {
	return 0
}

func LoadOffsetCoeff(src *byte) (offset int64, coeff float64) {
	return 0, 0
}
