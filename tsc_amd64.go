package tsc

import (
	"math"
	"time"

	"github.com/templexxx/cpu"
)

// Calibration configuration.
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
			for i := 0; i < 1000; i++ {
				_ = unixNanoTSCFMA()
			}
			fmaCost := GetInOrder() - start
			start = GetInOrder()
			for i := 0; i < 1000; i++ {
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

	// Invariant TSC ensures TSC counters stay synchronized across CPUs:
	// counters reset together and run at the same frequency.
	// In some VMs, CPUID may not expose 0x80000007. In that case,
	// allow TSC if the active Linux clock source is already "tsc".
	if !cpu.X86.HasInvariantTSC {
		if GetCurrentClockSource() != "tsc" {
			return false // Cannot confirm Invariant TSC from CPUID or Linux clock source.
		}
	}

	// Some instructions need AVX, see tsc_amd64.s for details.
	// We also rely on AVX for 16-byte atomic store/load in this implementation.
	// It is rare to find a modern CPU with Invariant TSC but without AVX.
	if !cpu.X86.HasAVX {
		return false
	}

	supported = 1
	return true
}

// Calibrate updates the mapping from TSC ticks to wall clock.
//
// Running this periodically is recommended (for example every 5 minutes).
func Calibrate() {

	if !isHardwareSupported() {
		return
	}

	cnt := samples

	tscs := make([]float64, cnt*2)
	syss := make([]float64, cnt*2)

	for j := 0; j < cnt; j++ {
		_, tsc0, sys0 := getClosestTSCSys(getClosestTSCSysRetries)
		time.Sleep(sampleDuration)
		_, tsc1, sys1 := getClosestTSCSys(getClosestTSCSysRetries)

		tscs[j*2] = float64(tsc0)
		tscs[j*2+1] = float64(tsc1)

		syss[j*2] = float64(sys0)
		syss[j*2+1] = float64(sys1)
	}

	coeff, offset := simpleLinearRegression(tscs, syss)
	storeOffsetCoeff(OffsetCoeffAddr, offset, coeff)
	storeOffsetFCoeff(OffsetCoeffFAddr, float64(offset), coeff)
}

func simpleLinearRegression(tscs, syss []float64) (coeff float64, offset int64) {

	tmean, wmean := float64(0), float64(0)
	for _, i := range tscs {
		tmean += i
	}
	for _, i := range syss {
		wmean += i
	}
	tmean = tmean / float64(len(tscs))
	wmean = wmean / float64(len(syss))

	denominator, numerator := float64(0), float64(0)
	for i := range tscs {
		numerator += (tscs[i] - tmean) * (syss[i] - wmean)
		denominator += math.Pow(tscs[i]-tmean, 2)
	}

	coeff = numerator / denominator

	return coeff, int64(wmean - coeff*tmean)
}

// CalibrateWithCoeff calibrates with a fixed coefficient provided by caller.
//
// Not thread-safe. Intended for tests only.
func CalibrateWithCoeff(c float64) {

	if !Supported() {
		return
	}

	_, tsc, sys := getClosestTSCSys(getClosestTSCSysRetries)
	off := sys - int64(float64(tsc)*c)
	storeOffsetCoeff(OffsetCoeffAddr, off, c)
	storeOffsetFCoeff(OffsetCoeffFAddr, float64(off), c)
}

// getClosestTSCSys finds the TSC value that is closest to a sampled system clock.
func getClosestTSCSys(n int) (minDelta, tscClock, sys int64) {

	// A loop size like 256 is usually enough to observe a low-overhead system clock sample.
	// Although time.Now() may use VDSO, latency still varies and can exceed 1000ns.
	// The larger loop improves sample quality while keeping calibration overhead low.
	// [tscClock, wc, tscClock, wc, ..., tscClock]
	timeline := make([]int64, n+n+1)

	timeline[0] = RDTSC()
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = time.Now().UnixNano()
		timeline[i+1] = RDTSC()
	}

	// minDelta is the smallest gap between adjacent TSC reads,
	// which also indicates the closest system-clock sample pairing.
	minDelta = int64(math.MaxInt64)
	minIndex := 1 // sys clock index corresponding to minDelta.

	// On some platforms (for example macOS), time.Now() may have microsecond granularity.
	// This creates repeated system-clock values in timeline; the middle sample is a better estimate.
	// Find min delta where the system clock is centered in a repeated-value window.
	for i := 1; i < len(timeline)-1; i += 2 {
		last := timeline[i]
		for j := i + 2; j < len(timeline)-1; j += 2 {
			if timeline[j] != last {
				mid := (i + j - 2) >> 1
				if isEven(mid) {
					mid++
				}

				delta := timeline[mid+1] - timeline[mid-1]
				if delta < minDelta {
					minDelta = delta
					minIndex = mid
				}

				i = j
				last = timeline[j]
			}
		}
	}

	tscClock = (timeline[minIndex+1] + timeline[minIndex-1]) >> 1
	sys = timeline[minIndex]

	return
}

// GetInOrder gets TSC in strict order.
// Used by calibration paths to avoid out-of-order effects.
//
//go:noescape
func GetInOrder() int64

// RDTSC gets TSC without ordering guarantees.
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

// Same logic as unixNanoTSC16B; used to verify offset and coeff loading.
//
//go:noescape
func LoadOffsetCoeff(src *byte) (offset int64, coeff float64)
