package tsc

import (
	"math"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/templexxx/cpu"
)

var (
	// padding for reducing cache pollution.
	_padding0 = cpu.X86FalseSharingRange
	offset    int64 // offset + toNano(tsc) = unix nano
	_padding1       = cpu.X86FalseSharingRange
	// Coeff (coefficient) * tsc = nano seconds.
	// Coeff is the inverse of TSCFrequency(GHz)
	// for avoiding future dividing.
	// MUL gets much better perf than DIV.
	//
	// Using an uint64 for atomic operation.
	Coeff     uint64 = 0
	_padding2        = cpu.X86FalseSharingRange
)

func init() {
	Enabled = enableTSC()
	if Enabled {

		var minDelta, minTsc, minWall uint64
		minDelta = math.MaxUint64
		for i := 0; i < 256; i++ { // Try to find the best one.
			md, tsc, wall := fastCalibrate()
			if md < minDelta {
				minDelta = md
				minTsc = tsc
				minWall = wall
			}
		}

		setOffset(minWall, minTsc)
		unixNano = unixNanoTSC
	}
}

func setOffset(ns, tsc uint64) {
	c := atomic.LoadUint64(&Coeff)
	off := ns - uint64(float64(tsc)*math.Float64frombits(c))
	atomic.StoreInt64(&offset, int64(off))
}

// unixNanoTSC returns unix nano time by TSC register.
//go:noescape
func unixNanoTSC() int64

// FreqEnv is the TSC frequency calculated by tools/getfreq or other tool.
// It'll help
const FreqEnv = "TSC_FREQ_X"

// enable TSC or not.
func enableTSC() bool {
	// Invariant TSC could make sure TSC got synced among multi CPUs.
	// They will be reset at same time, and run in same frequency.
	if !cpu.X86.HasInvariantTSC {
		return false
	}

	// Cannot get TSC frequency by CPU feature detection, may caused by:
	// 1. New CPU which I haven't updated the its crystal frequency. Please raise a issue, thank you.
	// 2. Virtual environment
	if cpu.X86.TSCFrequency == 0 {
		return false
	}

	// Some instructions need AVX, see tsc_amd64.s for details.
	// Actually, it's hardly to find a CPU without AVX supports in present. :)
	// And it's weird that a CPU has invariant TSC but doesn't have AVX.vbfg
	if !cpu.X86.HasAVX {
		return false
	}

	freq := fpFromEnv(FreqEnv)
	FreqSource = EnvSource
	if freq == 0 {
		freq = float64(cpu.X86.TSCFrequency)
		FreqSource = CPUFeatureSource
	}

	// The frequency provided by Intel manual is not that reliable,
	// (Actually, there is 1/millions delta at least)
	// it's easy to ensure that, because it's common that crystal will have "waves".
	// Yes, we can get an expensive crystal, but we can't replace the crystal in CPU
	// by the better one.
	// That's why we have to adjust the frequency by tools provided by this project.
	//
	// But we still need the frequency because it will be the bench for adjusting.
	c := math.Float64bits(1 / (freq / 1e9))
	atomic.StoreUint64(&Coeff, c)

	if Coeff == 0 {
		return false
	}

	return true
}

func fpFromEnv(name string) float64 {
	s := os.Getenv(name)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// Calibrate calibrates tsc & wall clock.
//
// It's a good practice that run Calibrate periodically (every 10-15mins) outside,
// because the wall clock may be calibrated (e.g. NTP).
//
// If !enabled do nothing.
func Calibrate() {

	if !Enabled {
		return
	}

	_, tsc, wall := fastCalibrate()
	setOffset(wall, tsc)
}

// fastCalibrate calibrates tsc clock and wall clock fastly,
// it's used for first checking and catching up wall clock adjustment.
//
// It will get clocks repeatedly, and try find the closest tsc clock and wall clock.
func fastCalibrate() (minDelta, tsc, wall uint64) {

	// 256 is enough for finding lowest wall clock cost in most cases.
	// Although time.Now() is using VDSO to get time, but it's unstable,
	// sometimes it will take more than 1000ns,
	// we have to use a big loop(e.g. 256) to get the "real" clock.
	// And it won't take a long time to finish the calibrate job, only about 20µs.
	n := 256
	// [tsc, wc, tsc, wc, ..., tsc]
	timeline := make([]uint64, n+n+1)

	timeline[0] = RDTSC() // TODO try to use not order
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = uint64(time.Now().UnixNano())
		timeline[i+1] = RDTSC()
	}

	// The minDelta is the smallest gap between two adjacent tscs,
	// which means the smallest gap between wall clock and tsc too.
	minDelta = uint64(math.MaxUint64)
	minIndex := 1 // minIndex is wall clock index where has minDelta.

	// time.Now()'s precision is only µs (on MacOS),
	// which means we will get multi same wall clock in timeline,
	// and the middle one is closer to the real time in statistics.
	// Try to find the minimum delta when wall clock is in the "middle".
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

	tsc = (timeline[minIndex+1] + timeline[minIndex-1]) >> 1
	wall = timeline[minIndex]

	return
}

// GetInOrder gets tsc value in strictly order.
// It's used for helping calibrate to avoid out-of-order issues.
//go:noescape
func GetInOrder() uint64

// RDTSC gets tsc value out-of-order.
//go:noescape
func RDTSC() uint64
