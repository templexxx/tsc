package tsc

import (
	"math"
	"os"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/templexxx/tsc/internal/xbytes"

	"github.com/templexxx/cpu"
)

var (
	// coeffOffset is a 16 bytes slice:
	// |          |          |
	// 0         8B          16B
	// |   coeff |   offset  |
	// Combine them together for atomic operation, because they are a pair must be used in the same function call.
	//
	// coeff (coefficient) * tsc_register + offset = unix_timestamp_nano_seconds.
	// coeff is a float64, offset is an int64.
	//
	// We could regard coeff as the inverse of TSCFrequency(GHz) (actually it just has mathematics property)
	// for avoiding future dividing.
	// MUL gets much better performance than DIV.
	coeffOffset = xbytes.MakeAlignedBlock(16, 64)
)

var (
	// padding for reducing cache pollution.
	_      [cpu.X86FalseSharingRange]byte
	Offset int64
	_      [cpu.X86FalseSharingRange]byte

	Frequency float64 = 0 // TSC frequency.

	coeff float64 = 0
	_     [cpu.X86FalseSharingRange]byte
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
		FreqSource = ""
		UnixNano = sysClock
		return false
	}
}

// SetFreq sets frequency manually.
func SetFreq(freq float64) {
	Frequency = freq
	c := 1 / (freq / 1e9)
	coeff = c
}

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
	if c == 0 { // Just in case.
		return false
	}

	coeff = c

	var minDelta, minTsc, minWall int64
	minDelta = math.MaxInt64
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

const acceptDelta = 10000 // 10us.

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

func setOffset(ns, tsc int64) {
	off := ns - int64(float64(tsc)*coeff)
	atomic.StoreInt64(&Offset, off)
}

type tscWall struct {
	tscc int64
	wall int64
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

// CalibrateWithCoeffOffset calibrates coefficient & offset to wall_clock by variables.
func CalibrateWithCoeffOffset(c float64, offset int64) {

	if !Enabled() {
		return
	}

	coeff = c
	Frequency = 1e9 / coeff
	atomic.StoreInt64(&Offset, offset)
}

// fastCalibrate calibrates tsc clock and wall clock in a fast way,
// it's used for first checking and catching up wall clock adjustment.
//
// It will get clocks repeatedly, and try find the closest tsc clock and wall clock.
func fastCalibrate() (minDelta, tsc, wall int64) {

	// 256 is enough for finding lowest wall clock cost in most cases.
	// Although time.Now() is using VDSO to get time, but it's unstable,
	// sometimes it will take more than 1000ns,
	// we have to use a big loop(e.g. 256) to get the "real" clock.
	// And it won't take a long time to finish the calibrate job, only about 20µs.
	n := 256
	// [tsc, wc, tsc, wc, ..., tsc]
	timeline := make([]int64, n+n+1)

	timeline[0] = RDTSC() // TODO try to use not order
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = time.Now().UnixNano()
		timeline[i+1] = RDTSC()
	}

	// The minDelta is the smallest gap between two adjacent tscs,
	// which means the smallest gap between wall clock and tsc too.
	minDelta = int64(math.MaxInt64)
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
