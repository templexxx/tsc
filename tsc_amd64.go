package tsc

import (
	"math"
	"os"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/templexxx/cpu"
)

var (
	// padding for reducing cache pollution.
	_padding0 = cpu.X86FalseSharingRange
	offset    int64 // offset + toNano(tsc) = unix nano
	_padding1 = cpu.X86FalseSharingRange

	Frequency float64 = 0	// TSC frequency.
	// coeff (coefficient) * tsc = nano seconds.
	// coeff is the inverse of TSCFrequency(GHz)
	// for avoiding future dividing.
	// MUL gets much better performance than DIV.
	coeff float64 = 0

	_padding2 = cpu.X86FalseSharingRange
)

var unixNano = unixNanoTSC

func init() {

	_ = reset()
}

func reset() bool {
	if enableTSC() {
		enabled = 1
		return true
	} else {
		enabled = 0
		FreqSource = ""
		unixNano = time.Now().UnixNano
		return false
	}
}

// unixNanoTSC returns unix nano time by TSC register.
//go:noescape
func unixNanoTSC() int64

// enable TSC or not.
func enableTSC() bool {

	if !isHardwareSupported() {
		return false
	}

	freq := fpFromEnv(FreqEnv) // Try get frequency from environment firstly.
	if freq != 0 {
		FreqSource = EnvSource
	} else {
		freq = getFreqNonEnv()
	}

	if freq == 0 {
		FreqSource = ""
		return false
	}

	Frequency = freq

	c := 1 / (freq / 1e9)
	if c == 0 {	// Just in case.
		return false
	}

	coeff = c

	var minDelta, minTsc, minWall uint64
	minDelta = math.MaxUint64
	for i := 0; i < 1024; i++ { // Try to find the best one.
		md, tsc, wall := fastCalibrate()
		if md < minDelta {
			minDelta = md
			minTsc = tsc
			minWall = wall
		}
	}

	setOffset(minWall, minTsc)

	pass := checkDelta()
	if pass {
		stable = 1
	} else {
		stable = 0
	}

	if forceTSC == 1 {
		return true
	}

	if !pass {
		FreqSource = ""
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
	// Actually, it's hardly to find a CPU without AVX supports in present. :)
	// And it's weird that a CPU has invariant TSC but doesn't have AVX.
	if !cpu.X86.HasAVX {
		return false
	}

	supported = 1
	return true
}

func getFreqNonEnv() float64 {
	ff := getFreqFast()
	if ff != 0 {
		FreqSource = FastDetectSource
		return ff
	}

	cf := float64(cpu.X86.TSCFrequency)
	if cf != 0 {
		FreqSource = CPUFeatureSource
		return cf
	}

	return 0
}

const acceptDelta = 10000	// 10us.

// checkDelta checks tsc clock & system clock delta in a fast way.
// Expect < 10us/s.
// Return true if pass.
func checkDelta() bool {

	time.Sleep(time.Second)
	tscc := unixNanoTSC()
	wallc := time.Now().UnixNano()

	if math.Abs(float64(tscc-wallc)) > acceptDelta { // Which means every 1s has > 10us delta, too much.
		return false
	}
	return true
}

func setOffset(ns, tsc uint64) {
	off := ns - uint64(float64(tsc)*coeff)
	atomic.StoreInt64(&offset, int64(off))
}

type tscWall struct {
	tscc uint64
	wall uint64
}

// getFreqFast gets tsc frequency with a fast detection.
func getFreqFast() float64 {

	round := 16
	freqs := make([]float64, round-1)
	ret := make([]tscWall, round)
	for k := 0; k < round; k++ {
		_, tscc, wallc := fastCalibrate()
		ret[k] = tscWall{tscc: tscc, wall: wallc}
		time.Sleep(time.Millisecond)
	}

	for i := 1; i < round; i++ {
		freq := float64(ret[i].tscc-ret[i-1].tscc) * 1e9 / float64(ret[i].wall-ret[i-1].wall)
		freqs[i-1] = freq
	}

	sort.Float64s(freqs)
	freqs = freqs[1:]
	freqs = freqs[:len(freqs)-1]

	totalFreq := float64(0)
	for i := range freqs {
		totalFreq += freqs[i]
	}

	return totalFreq / float64(len(freqs))
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

	if !Enabled() {
		return
	}

	_, tsc, wall := fastCalibrate()
	setOffset(wall, tsc)
}

// fastCalibrate calibrates tsc clock and wall clock in a fast way,
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
