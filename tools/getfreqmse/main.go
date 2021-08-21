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
	sample = flag.Int("sample", 1024, "number of samples")
	frange = flag.Int("range", 1024, "after getting avg frequency, try [avg-range, avg+range] to get MSE")
	drop   = flag.Int("drop", 10, "drop x% min & max in samples")
)

func main() {
	flag.Parse()

	if !tsc.Supported() {
		fmt.Println("tsc unsupported")
		return
	}

	tsc.ForceTSC() // Enable TSC force.

	start := time.Now()

	cnt := *sample

	if cnt < 128 {
		cnt = 128 // TODO at least 128 is a good choice?
	}

	freqsOneStep := make([]float64, cnt-1) // Frequency is calculated by two adjacent samples (one step each).
	freqsSteps := make([]float64, cnt-1)   // Frequency is calculated by sample_i & sample_0 (i>=1) pair (i step each).

	samples := make([]tscWall, cnt)
	for i := 0; i < cnt; i++ {
		var minDelta, minTsc, minWall uint64
		minDelta = math.MaxUint64
		for j := 0; j < 256; j++ { // Try to find the best one inside 256 tries (avoiding jitter).
			md, tscc, wallc := calibrate(256)
			if md < minDelta {
				minDelta = md
				minTsc = tscc
				minWall = wallc
			}
		}
		samples[i] = tscWall{tscc: minTsc, wall: minWall}
	}

	for i := 1; i < cnt; i++ {
		freq := float64(samples[i].tscc-samples[i-1].tscc) / float64(samples[i].wall-samples[i-1].wall)
		freqsOneStep[i-1] = freq * 1e9

		freq = float64(samples[i].tscc-samples[0].tscc) / float64(samples[i].wall-samples[0].wall)
		freqsSteps[i-1] = freq * 1e9
	}

	avgFreq0, mseFreq0 := calcMSE(freqsOneStep, *frange, *drop, oneStep, samples)
	avgFreq1, mseFreq1 := calcMSE(freqsSteps, *frange, *drop, steps, samples)

	cost := time.Now().Sub(start)

	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	fmt.Printf("cpu: %s, job cost: %.2fs\n", cpuFlag, cost.Seconds())
	fmt.Println("-------")
	fmt.Printf("origin freq is: %.9f\n", tsc.Frequency)
	fmt.Println("=======")
	report(oneStep, avgFreq0, mseFreq0)
	report(steps, avgFreq1, mseFreq1)
}

func report(step int, avg, mse float64) {
	stepFmt := "one_step"
	if step != oneStep {
		stepFmt = "steps"
	}
	fmt.Printf("steps: %s, avg: %.9f, mse: %.9f\n", stepFmt, avg, mse)
}

const (
	oneStep = iota
	steps
)

func calcMSE(freqs []float64, fr, drop, step int, samples []tscWall) (avgFreq, mseFreq float64) {

	sort.Float64s(freqs)
	dropCnt := int(float64(drop) / 100 * float64(len(freqs)))
	freqs = freqs[dropCnt:]
	freqs = freqs[:len(freqs)-dropCnt]

	var total float64
	for _, f := range freqs {
		total += f
	}
	avgFreq = total / float64(len(freqs))

	mse := math.MaxFloat64

	for f := avgFreq - float64(fr); f <= avgFreq+float64(fr); f++ {

		var mse0 float64
		switch step {
		case oneStep:
			for i := 1; i < len(samples); i++ {
				predict := float64(samples[i].wall-samples[i-1].wall) * (f / 1e9)
				delta := math.Abs(predict - (float64(samples[i].tscc - samples[i-1].tscc)))
				mse0 += math.Pow(delta, 2)
			}
		default: // steps
			for i := 1; i < len(samples); i++ {
				predict := float64(samples[i].wall-samples[0].wall) * (f / 1e9)
				delta := math.Abs(predict - (float64(samples[i].tscc - samples[0].tscc)))
				mse0 += math.Pow(delta, 2)
			}
		}
		if mse0 < mse {
			mse = mse0
			mseFreq = f
		}
	}
	return avgFreq, mseFreq
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
