//go:build arm64

package tsc

import (
	"time"
)

// Configs of calibration.
// See tools/calibrate for details.
const (
	samples                 = 128
	sampleDuration          = 16 * time.Millisecond
	getClosestTSCSysRetries = 256
)

// ARM64FalseSharingRange is the cache line size on ARM64 (typically 64 bytes)
const ARM64FalseSharingRange = 64

func init() {
	_ = reset()
}

func reset() bool {
	if !isHardwareSupported() {
		return false
	}

	Calibrate()

	if IsOutOfOrder() {
		// Try to determine if FMADD variant is faster
		start := GetInOrder()
		for i := 0; i < 1000; i++ {
			_ = unixNanoARMFMADD()
		}
		fmaCost := GetInOrder() - start

		start = GetInOrder()
		for i := 0; i < 1000; i++ {
			_ = unixNanoARM16B()
		}
		armCost := GetInOrder() - start

		if fmaCost < armCost {
			UnixNano = unixNanoARMFMADD
		} else {
			UnixNano = unixNanoARM16B
		}
		return true
	}

	UnixNano = unixNanoARM16Bfence
	return true
}

func isHardwareSupported() bool {
	if supported == 1 {
		return true
	}

	// Read the counter frequency register
	freq := readCounterFrequency()
	if freq == 0 {
		// If we can't read the frequency, check Linux clock source
		if GetCurrentClockSource() != "arch_sys_counter" {
			return false
		}
	}

	// ARM64 Generic Timer should be available on all arm64 systems
	// NEON is standard on ARM64, so we don't need explicit checks

	supported = 1
	return true
}

// Calibrate calibrates the ARM Generic Timer clock using hybrid approach.
//
// It's a good practice that runs Calibrate periodically (e.g., 5 min is a good start).
func Calibrate() {
	if !isHardwareSupported() {
		return
	}

	// Hybrid approach: Use frequency from hardware, refine with linear regression
	// The ARM64 Generic Timer provides frequency via CNTFRQ_EL0
	// We use linear regression to measure actual drift and get accurate offset

	// Run linear regression calibration for accuracy
	cnt := samples

	tscs := make([]float64, cnt*2)
	syss := make([]float64, cnt*2)

	for j := 0; j < cnt; j++ {
		tsc0, sys0 := getClosestTSCSys(getClosestTSCSysRetries)
		time.Sleep(sampleDuration)
		tsc1, sys1 := getClosestTSCSys(getClosestTSCSysRetries)

		tscs[j*2] = float64(tsc0)
		tscs[j*2+1] = float64(tsc1)

		syss[j*2] = float64(sys0)
		syss[j*2+1] = float64(sys1)
	}

	coeff, offset := simpleLinearRegression(tscs, syss)

	// Store calibrated values
	storeOffsetCoeff(OffsetCoeffAddr, offset, coeff)
	storeOffsetFCoeff(OffsetCoeffFAddr, float64(offset), coeff)
}

// CalibrateWithCoeff calibrates coefficient to wall_clock by variables.
//
// Not thread safe, only for testing.
func CalibrateWithCoeff(c float64) {
	if !Supported() {
		return
	}

	tsc, sys := getClosestTSCSys(getClosestTSCSysRetries)
	off := sys - int64(float64(tsc)*c)
	storeOffsetCoeff(OffsetCoeffAddr, off, c)
	storeOffsetFCoeff(OffsetCoeffFAddr, float64(off), c)
}

// GetInOrder gets counter value in strict order.
// It's used to help calibrating to avoid out-of-order issues.
//
//go:noescape
func GetInOrder() int64

// RDTSC gets counter value out-of-order (fast path).
//
//go:noescape
func RDTSC() int64

// readCounterFrequency reads the CNTFRQ_EL0 register.
//
//go:noescape
func readCounterFrequency() int64

//go:noescape
func unixNanoARM16B() int64

//go:noescape
func unixNanoARMFMADD() int64

//go:noescape
func unixNanoARM16Bfence() int64

//go:noescape
func storeOffsetCoeff(dst *byte, offset int64, coeff float64)

//go:noescape
func storeOffsetFCoeff(dst *byte, offset, coeff float64)

// LoadOffsetCoeff loads offset & coeff for checking.
//
//go:noescape
func LoadOffsetCoeff(src *byte) (offset int64, coeff float64)
