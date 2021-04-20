package main

import (
	"flag"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/templexxx/cpu"

	"github.com/templexxx/tsc"
)

var (
	round = flag.Int64("round", 60, "job rounds")
)

func main() {
	flag.Parse()

	if !tsc.Enabled {
		fmt.Println("tsc unsupported")
		return
	}

	start := time.Now()

	cnt := *round * 1024
	ret := make([]tscWall, cnt)
	for i := 0; i < int(cnt); i++ {
		var minDelta, minTsc, minWall uint64
		minDelta = math.MaxUint64
		for j := 0; j < 256; j++ { // Try to find the best one.
			md, tscc, wallc := calibrate(256)
			if md < minDelta {
				minDelta = md
				minTsc = tscc
				minWall = wallc
			}
		}
		ret[i] = tscWall{tscc: minTsc, wall: minWall}
	}

	deltas := make([]tscWallDelta, cnt-1)
	freqs := make([]float64, cnt-1)
	for i := 1; i < int(cnt); i++ {
		deltas[i-1] = tscWallDelta{tscDelta: ret[i].tscc - ret[i-1].tscc,
			wallDelta: ret[i].wall - ret[i-1].wall}
		deltas[i-1].freq = float64(deltas[i-1].tscDelta) * 1e9 / float64(deltas[i-1].wallDelta)
		freqs[i-1] = deltas[i-1].freq
	}

	sort.Float64s(freqs)

	freqs = freqs[128:]
	freqs = freqs[:len(freqs)-128]

	totalFreq := float64(0)
	for i := range freqs {
		totalFreq += freqs[i]
	}

	cost := time.Now().Sub(start)

	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	fmt.Printf("cpu: %s freq: %.9f, cost: %.2fs\n", cpuFlag, totalFreq/float64(len(freqs)), cost.Seconds())
}

type tscWallDelta struct {
	tscDelta  uint64
	wallDelta uint64
	freq      float64
}

type tscWall struct {
	tscc uint64
	wall uint64
}

func calibrate(n int) (minDelta, tscClock, wall uint64) {

	// 256 is enough for finding lowest wall clock cost in most cases.
	// Although time.Now() is using VDSO to get time, but it's unstable,
	// sometimes it will take more than 1000ns,
	// we have to use a big loop(e.g. 256) to get the "real" clock.
	// And it won't take a long time to finish the calibrate job, only about 20µs.
	// [tscClock, wc, tscClock, wc, ..., tscClock]
	timeline := make([]uint64, n+n+1)

	timeline[0] = tsc.RDTSC()
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = uint64(time.Now().UnixNano())
		timeline[i+1] = tsc.RDTSC()
	}

	// The minDelta is the smallest gap between two adjacent tscs,
	// which means the smallest gap between wall clock and tscClock too.
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

	tscClock = (timeline[minIndex+1] + timeline[minIndex-1]) >> 1
	wall = timeline[minIndex]

	return
}

func isEven(n int) bool {
	return n&1 == 0
}
