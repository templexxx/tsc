package tsc

import (
	"math"
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
			UnixNano = unixNanoTSCFMA
			return true
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
	// They will be reset at same time, and run in same frequency.
	if !cpu.X86.HasInvariantTSC {
		return false
	}

	// Some instructions need AVX, see tsc_amd64.s for details.
	// And we need AVX supports for 16 Bytes atomic store/load, see internal/xatomic for deatils.
	// Actually, it's hardly to find a CPU without AVX supports in present. :)
	// And it's weird that a CPU has invariant TSC but doesn't have AVX.
	if !cpu.X86.HasAVX {
		return false
	}

	supported = 1
	return true
}

// Calibrate calibrates tsc clock.
//
// It's a good practice that run Calibrate periodically (every 10-15mins),
// because the system clock may be calibrated by network (e.g. NTP).
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
	for i := range tscs {
		tmean += tscs[i]
		wmean += syss[i]
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

// CalibrateWithCoeff calibrates coefficient to wall_clock by variables.
//
// Not thread safe, only for testing.
func CalibrateWithCoeff(c float64) {

	if !Supported() {
		return
	}

	_, tsc, sys := getClosestTSCSys(getClosestTSCSysRetries)
	off := sys - int64(float64(tsc)*c)
	storeOffsetCoeff(OffsetCoeffAddr, off, c)
	storeOffsetFCoeff(OffsetCoeffFAddr, float64(off), c)
}

// getClosestTSCSys tries to get the closest tsc register value nearby the system clock in a loop.
func getClosestTSCSys(n int) (minDelta, tscClock, sys int64) {

	// 256 is enough for finding the lowest sys clock cost in most cases.
	// Although time.Now() is using VDSO to get time, but it's unstable,
	// sometimes it will take more than 1000ns,
	// we have to use a big loop(e.g. 256) to get the "real" clock.
	// And it won't take a long time to finish calibrating job, only about 20µs.
	// [tscClock, wc, tscClock, wc, ..., tscClock]
	timeline := make([]int64, n+n+1)

	timeline[0] = GetInOrder()
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = time.Now().UnixNano()
		timeline[i+1] = GetInOrder()
	}

	// The minDelta is the smallest gap between two adjacent tscs,
	// which means the smallest gap between sys clock and tscClock too.
	minDelta = int64(math.MaxInt64)
	minIndex := 1 // minIndex is sys clock index where has minDelta.

	// time.Now()'s precision is only µs (on macOS),
	// which means we will get multi same sys clock in timeline,
	// and the middle one is closer to the real time in statistics.
	// Try to find the minimum delta when sys clock is in the "middle".
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

// GetInOrder gets tsc value in strictly order.
// It's used for helping calibrate to avoid out-of-order issues.
//go:noescape
func GetInOrder() int64

// RDTSC gets tsc value out-of-order.
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
//go:noescape
func LoadOffsetCoeff(src *byte) (offset int64, coeff float64)
