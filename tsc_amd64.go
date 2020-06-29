package tsc

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/templexxx/cpu"
)

var (
	offset int64 // offset + toNano(tsc) = unix nano.

	// coeff(coefficient) * tsc = nano seconds.
	// coeff is the inverse of TSCFrequency(GHz)
	// for avoiding future dividing.
	// MUL gets much better perf than DIV.
	coeff float64 = 0
)

func init() {
	Enabled = enableTSC()
	if Enabled {
		calibrate()
		unixNano = unixNanoTSC
	}
}

//go:noescape
func unixNanoTSC() int64

// enable TSC or not.
func enableTSC() bool {
	// Invariant TSC could make sure TSC got synced among multi CPUs.
	// They will be reset at same time, and run in same frequency.
	if !cpu.X86.HasInvariantTSC {
		return false
	}

	if cpu.X86.TSCFrequency == 0 {
		return false
	}

	if !cpu.X86.HasAVX { // Some instructions need AVX, see tsc_amd64.s for details.
		return false
	}

	coeff = 1 / (float64(cpu.X86.TSCFrequency) / 1e9)

	if coeff == 0 {
		return false
	}

	return true
}

// Calibrate calibrates tsc & wall clock.
//
// It's a good practice that run Calibrate periodically (every hour) outside,
// because the wall clock may be calibrated (e.g. NTP).
//
// If !enabled do nothing.
func Calibrate() {

	if !Enabled {
		return
	}

	calibrate()
}

func calibrate() {

	// 1024 is enough for finding lowest wall clock cost in most cases.
	// Although time.Now() is using VDSO to get time, but it's unstable,
	// sometimes it will take more than 1000ns,
	// we have to use a big loop(e.g. 1024) to get the "real" clock.
	n := 1024
	timeline := make([]uint64, n+n+1)

	timeline[0] = getInOrder()
	// [tsc, wc, tsc, wc, ..., tsc]
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = uint64(time.Now().UnixNano())
		timeline[i+1] = getInOrder()
	}

	// The minDelta is the smallest gap between two adjacent tscs,
	// which means the smallest gap between wall clock and tsc too.
	minDelta := uint64(math.MaxUint64)
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

	tsc := (timeline[minIndex+1] + timeline[minIndex-1]) >> 1
	wall := timeline[minIndex]

	// Use atomic to protect offset in periodically Calibrate.
	atomic.StoreInt64(&offset, int64(wall)-toNano(tsc))
}

// toNano converts tsc to nanoseconds.
//
// Returns 0 if nothing to do.
func toNano(tsc uint64) int64 {
	return int64(float64(tsc) * coeff)
}

// getInOrder gets tsc value in strictly order.
// It's used for helping calibrate to avoid out-of-order issues.
//go:noescape
func getInOrder() uint64
