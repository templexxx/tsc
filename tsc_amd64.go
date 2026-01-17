package tsc

import (
	"time"

	"github.com/templexxx/cpu"
)

// Configs of calibration.
// See tools/calibrate for details.
const (
	samples                 = 128
	sampleDuration          = 16 * time.Millisecond
	getClosestTSCSysRetries = 256
)

func init() {
	_ = reset()
}

func reset() bool {
	if !isHardwareSupported() {
		return false
	}

	Calibrate()

	if IsOutOfOrder() {
		if cpu.X86.HasFMA {
			start := GetInOrder()

			for range 1000 {
				_ = unixNanoTSCFMA()
			}

			fmaCost := GetInOrder() - start
			start = GetInOrder()

			for range 1000 {
				_ = unixNanoTSC16B()
			}

			tscCost := GetInOrder() - start
			if fmaCost < tscCost {
				UnixNano = unixNanoTSCFMA
			}
		}

		UnixNano = unixNanoTSC16B

		return true
	}

	UnixNano = unixNanoTSC16Bfence

	return true
}

func isHardwareSupported() bool {
	if supported == 1 {
		return true
	}

	// Invariant TSC could make sure TSC got synced among multi CPUs.
	// They will be reset at the same time and run the same frequency.
	// But in some VM, the max Extended Function in CPUID is < 0x80000007;
	// we should enable TSC if the system clock source is TSC.
	if !cpu.X86.HasInvariantTSC {
		if GetCurrentClockSource() != "tsc" {
			return false // Cannot detect invariant tsc by CPUID or linux clock source, return false.
		}
	}

	// Some instructions need AVX, see tsc_amd64.s for details.
	// And we need AVX support for 16 Bytes atomic store/load, see internal/xatomic for details.
	// Actually, it's hard to find a CPU without AVX support at present. :)
	// And it's unique that a CPU has invariant TSC but doesn't have AVX.
	if !cpu.X86.HasAVX {
		return false
	}

	supported = 1

	return true
}

// Calibrate calibrates tsc clock.
//
// It's a good practice that runs Calibrate periodically (e.g., 5 min is a good start).
func Calibrate() {
	if !isHardwareSupported() {
		return
	}

	cnt := samples

	tscs := make([]float64, cnt*2)
	syss := make([]float64, cnt*2)

	for j := range cnt {
		tsc0, sys0 := getClosestTSCSys(getClosestTSCSysRetries)

		time.Sleep(sampleDuration)

		tsc1, sys1 := getClosestTSCSys(getClosestTSCSysRetries)

		tscs[j*2] = float64(tsc0)
		tscs[j*2+1] = float64(tsc1)

		syss[j*2] = float64(sys0)
		syss[j*2+1] = float64(sys1)
	}

	coeff, offset := simpleLinearRegression(tscs, syss)
	storeOffsetCoeff(OffsetCoeffAddr, offset, coeff)
	storeOffsetFCoeff(OffsetCoeffFAddr, float64(offset), coeff)
}

// CalibrateWithCoeff calibrates coefficient to wall_clock by variables.
//
// Not thread safe, only for testing.
func CalibrateWithCoeff(coeff float64) {
	if !Supported() {
		return
	}

	tsc, sys := getClosestTSCSys(getClosestTSCSysRetries)
	off := sys - int64(float64(tsc)*coeff)
	storeOffsetCoeff(OffsetCoeffAddr, off, coeff)
	storeOffsetFCoeff(OffsetCoeffFAddr, float64(off), coeff)
}

// GetInOrder gets tsc value in strict order.
// It's used to help calibrating to avoid out-of-order issues.
//
//go:noescape
func GetInOrder() int64

// RDTSC gets tsc value out-of-order.
//
//go:noescape
func RDTSC() int64

//go:noescape
func unixNanoTSC16B() int64

//go:noescape
func unixNanoTSCFMA() int64

//go:noescape
func unixNanoTSC16Bfence() int64

//go:noescape
func storeOffsetCoeff(dst *byte, offset int64, coeff float64)

//go:noescape
func storeOffsetFCoeff(dst *byte, offset, coeff float64)

// Same logic as unixNanoTSC16B for checking getting offset & coeff correctly.
//
//go:noescape
func LoadOffsetCoeff(src *byte) (offset int64, coeff float64)
