package tsc

import (
	"math"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/templexxx/cpu"
)

// unix_nano_timestamp = tsc_register_value * Coeff + Offset.
// Coeff = 1 / (tsc_frequency / 1e9).
// We could regard coeff as the inverse of TSCFrequency(GHz) (actually it just has mathematics property)
// for avoiding future dividing.
// MUL gets much better performance than DIV.
//
// Although I separate Offset & Coeff, we could tolerate they are not a pair, it won't cause disaster.
var (
	_      [cpu.X86FalseSharingRange]byte
	Offset int64
	_      [cpu.X86FalseSharingRange]byte
	Coeff  uint64 = 0
	_      [cpu.X86FalseSharingRange]byte
)

// Configs of calibration.
// See tools/calibrate for details.
const (
	samples                 = 128
	sampleDuration          = 16 * time.Millisecond
	getClosestTSCSysRetries = 256
)

const (
	// 10us/s. It's within the crystal ppm.
	// Clock generators for Intel processors are typically specified to have an accuracy of no worse than 100 ppm, or +/- 0.01%.
	// https://community.intel.com/t5/Software-Tuning-Performance/TSC-frequency-variations-with-temperature/td-p/1098982
	acceptDelta = 10000
)

func init() {

	_ = reset()
}

func reset() bool {
	if enableTSC() {
		enabled = 1
		if IsOutOfOrder() {
			UnixNano = unixNanoTSC
		} else {
			UnixNano = unixNanoTSCfence
		}
		return true
	} else {
		enabled = 0
		UnixNano = sysClock
		return false
	}
}

// enable TSC or not.
func enableTSC() bool {

	if !isHardwareSupported() {
		return false
	}

	Calibrate()

	pass := isGoodClock()
	if pass {
		stable = 1
	} else {
		stable = 0
	}

	if forceTSC == 1 {
		return true
	}

	if !pass {
		return false
	}

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

	if !Enabled() || !isGoodClock() {
		return
	}

	cnt := samples

	tscDeltas := make([]float64, cnt)
	sysDeltas := make([]float64, cnt)

	var minDelta, minTSC, minSys int64
	minDelta = math.MaxInt64

	for j := 0; j < cnt; j++ {
		md0, tsc0, sys0 := getClosestTSCSys(getClosestTSCSysRetries)
		time.Sleep(sampleDuration)
		md1, tsc1, sys1 := getClosestTSCSys(getClosestTSCSysRetries)

		tscDeltas[j] = float64(tsc1 - tsc0)
		sysDeltas[j] = float64(sys1 - sys0)

		if md0 < minDelta {
			minDelta = md0
			minTSC = tsc0
			minSys = sys0
		}
		if md1 < minDelta {
			minDelta = md1
			minTSC = tsc1
			minSys = sys1
		}
	}

	trainSetCnt := int(float64(cnt) * 0.8)

	rand.Seed(time.Now().UnixNano())

	rand.Shuffle(cnt, func(i, j int) {
		tscDeltas[i], tscDeltas[j] = tscDeltas[j], tscDeltas[i]
		sysDeltas[i], sysDeltas[j] = sysDeltas[j], sysDeltas[i]
	})
	coeff, _ := simpleLinearRegression(tscDeltas[:trainSetCnt], sysDeltas[:trainSetCnt])
	atomic.StoreUint64(&Coeff, math.Float64bits(coeff))
	setOffset(minSys, minTSC)
}

// isGoodClock checks tsc clock & system clock delta in a fast way.
// Expect < 10us/s.
// Return true if pass.
func isGoodClock() bool {

	time.Sleep(time.Second)
	tscc := unixNanoTSC()
	sysc := time.Now().UnixNano()

	if math.Abs(float64(tscc-sysc)) > acceptDelta { // Which means every 1s has > 10us delta, too much.
		return false
	}
	return true
}

func setOffset(ns, tsc int64) {
	c := math.Float64frombits(atomic.LoadUint64(&Coeff))
	off := ns - int64(float64(tsc)*c)
	atomic.StoreInt64(&Offset, off)
}

// simpleLinearRegression without intercept:
// α = ∑𝑥𝑖𝑦𝑖 / ∑𝑥𝑖^2.
func simpleLinearRegression(tscs, syss []float64) (coeff float64, offset int64) {

	denominator, numerator := float64(0), float64(0)
	for i := range tscs {
		numerator += tscs[i] * syss[i]
		denominator += math.Pow(tscs[i], 2)
	}

	coeff = numerator / denominator

	return coeff, 0
}

// CalibrateWithCoeff calibrates coefficient to wall_clock by variables.
//
// Not thread safe, only for testing.
func CalibrateWithCoeff(c float64) {

	if !Enabled() {
		return
	}

	atomic.StoreUint64(&Coeff, math.Float64bits(c))
	_, tsc, wall := getClosestTSCSys(getClosestTSCSysRetries)
	setOffset(wall, tsc)
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

// unixNanoTSC returns unix nano time by TSC register. (May backward because no fence to protect the execution order)
//go:noescape
func unixNanoTSC() int64

// unixNanoTSCfence returns unix nano time by TSC register with fence protection(won't be order-of-order).
//go:noescape
func unixNanoTSCfence() int64
