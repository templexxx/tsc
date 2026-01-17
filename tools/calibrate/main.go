package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime"
	"time"

	"github.com/templexxx/cpu"
	"github.com/templexxx/tsc"
)

const (
	sleepDuration = time.Second
	// Actually it depends on tsc crystal frequency & speed of CPU.
	// I don't think userspace fast clock is meaningful on a slow machine.
	// For a machine with good enough hardware, the min tsc delta mustn't be big.
	minTSCDeltaMac     int64 = 800
	minTSCDeltaLinux   int64 = 800 // System clock's speed is faster on Linux.
	triesToFindClosest       = 256 // If too small, hard to ensure getting minTSCDelta.
	minSamples               = 32
)

func main() {
	sample := flag.Int("sample", 128,
		"number of samples, the more samples the better result (not just number of samples, the term gets longer too)")
	durationFlag := flag.Int64("duration", 16,
		"duration(ms) between two timestamp, we need a bit longer duration for make result better in long term")
	printSample := flag.Bool("print", false, "print every sample")
	withIntercept := flag.Bool("offset", false, "using simple linear regression with intercept to get offset")

	flag.Parse()

	if !tsc.Supported() {
		log.Fatal("tsc unsupported")
	}

	cnt := max(*sample, minSamples)

	simulateFuncName := "simple linear regression without intercept"
	if *withIntercept {
		simulateFuncName = "simple linear regression with intercept"
	}

	minTSCDelta := getMinTSCDelta()

	duration := time.Duration(*durationFlag) * time.Millisecond
	if duration == 0 {
		duration = sleepDuration
	}

	start := time.Now()

	freqs, tscDeltas, sysDeltas, tscs, syss := collectSamples(cnt, duration, minTSCDelta, *printSample)

	cost := time.Since(start)

	avgFreq, avgCoeff, avgOffset := calculateAvg(freqs, cnt, tscs, syss)

	printInitialResults(cost, avgCoeff, avgFreq)

	coeff, offset, trainSetCnt := runRegression(cnt, tscDeltas, sysDeltas, tscs, syss, *withIntercept)

	log.Printf("result of %s, coeff: %.16f, freq: %.16f, offset: %d(%s)\n",
		simulateFuncName, coeff, 1e9/coeff, offset, nanosFmt(offset))
	log.Println("-------")

	runPrediction(avgCoeff, avgOffset, coeff, offset, trainSetCnt,
		tscDeltas, sysDeltas, tscs, syss, *withIntercept, simulateFuncName)
}

func getMinTSCDelta() int64 {
	minTSCDelta := minTSCDeltaLinux
	if runtime.GOOS == `darwin` {
		minTSCDelta = minTSCDeltaMac
	} else if runtime.GOOS != `linux` {
		log.Fatalf("sorry, haven't been well tested on: %s", runtime.GOOS)
	}

	return minTSCDelta
}

func collectSamples(cnt int, duration time.Duration,
	minTSCDelta int64, printSample bool,
) ([]float64, []float64, []float64, []float64, []float64) {
	freqs := make([]float64, cnt)
	tscDeltas := make([]float64, cnt)
	sysDeltas := make([]float64, cnt)
	tscs := make([]float64, cnt*2)
	syss := make([]float64, cnt*2)

	for j := range cnt {
		md0, tscc0, sys0 := getClosestTSCSys(triesToFindClosest)
		if md0 > minTSCDelta {
			log.Fatalf("the min tsc delta too big, exp <= %d, but got: %d", minTSCDelta, md0)
		}

		time.Sleep(duration)

		md1, tscc1, sys1 := getClosestTSCSys(triesToFindClosest)
		if md1 > minTSCDelta {
			log.Fatalf("the min tsc delta too big, exp <= %d, but got: %d", minTSCDelta, md1)
		}

		if sys1-sys0 < int64(duration) {
			log.Fatalf("sys clock goes backwards, exp %s, but got: %.2fms",
				duration.String(), float64(sys1-sys0)/float64(time.Millisecond))
		}

		freq := (float64(tscc1-tscc0) / float64(sys1-sys0)) * 1e9
		freqs[j] = freq

		if printSample {
			log.Printf("round: %d freq is: %.9f, min tsc deltas are: %d, %d, duration of closest sysclock: %.2fms\n",
				j, freq, md0, md1, float64(sys1-sys0)/1000/1000)
		}

		tscDeltas[j] = float64(tscc1 - tscc0)
		sysDeltas[j] = float64(sys1 - sys0)

		tscs[j*2] = float64(tscc0)
		tscs[j*2+1] = float64(tscc1)

		syss[j*2] = float64(sys0)
		syss[j*2+1] = float64(sys1)
	}

	return freqs, tscDeltas, sysDeltas, tscs, syss
}

func calculateAvg(freqs []float64, cnt int, tscs, syss []float64) (float64, float64, int64) {
	avgFreq := float64(0)
	for _, f := range freqs {
		avgFreq += f
	}

	avgFreq /= float64(cnt)
	avgCoeff := 1 / (avgFreq / 1e9)
	avgOffset := int64(syss[cnt*2-1]) - int64(tscs[cnt*2-1]*avgCoeff)

	return avgFreq, avgCoeff, avgOffset
}

func printInitialResults(cost time.Duration, avgCoeff, avgFreq float64) {
	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	log.Printf("cpu: %s, job cost: %.2fs\n", cpuFlag, cost.Seconds())
	log.Println("-------")

	ooffset, ocoeff := tsc.LoadOffsetCoeff(tsc.OffsetCoeffAddr)

	log.Printf("origin coeffcient: %.16f, freq: %.16f, offset: %d(%s)\n", ocoeff, 1e9/ocoeff, ooffset, nanosFmt(ooffset))
	log.Printf("avg coeffcient: %.16f, freq: %.16f\n", avgCoeff, avgFreq)
	log.Println("-------")
}

func runRegression(cnt int, tscDeltas, sysDeltas, tscs, syss []float64, withIntercept bool) (float64, int64, int) {
	trainSetCnt := int(float64(cnt) * 0.8)

	var (
		coeff  float64
		offset int64
	)

	if !withIntercept {
		rand.Shuffle(cnt, func(i, j int) {
			tscDeltas[i], tscDeltas[j] = tscDeltas[j], tscDeltas[i]
			sysDeltas[i], sysDeltas[j] = sysDeltas[j], sysDeltas[i]
		})

		coeff, _ = simpleLinearRegression(tscDeltas[:trainSetCnt], sysDeltas[:trainSetCnt])
	} else {
		rand.Shuffle(cnt*2, func(i, j int) {
			tscs[i], tscs[j] = tscs[j], tscs[i]
			syss[i], syss[j] = syss[j], syss[i]
		})

		trainSetCnt *= 2
		coeff, offset = simpleLinearRegressionWithIntercept(tscs[:trainSetCnt], syss[:trainSetCnt])
	}

	return coeff, offset, trainSetCnt
}

func runPrediction(avgCoeff float64, avgOffset int64, coeff float64, offset int64,
	trainSetCnt int, tscDeltas, sysDeltas, tscs, syss []float64, withIntercept bool, simulateFuncName string,
) {
	avgPredictDelta := float64(0)
	totalPredictDelta := float64(0)

	avgPredictDeltaAvgFreq := float64(0)
	totalPredictDeltaAvgFreq := float64(0)

	if !withIntercept {
		for i := range tscDeltas[trainSetCnt:] {
			predict := int64(tscDeltas[i+trainSetCnt] * coeff)
			predictAvgFreq := int64(tscDeltas[i+trainSetCnt] * avgCoeff)

			act := sysDeltas[i+trainSetCnt]

			avgPredictDelta += math.Abs(float64(predict) - act)
			totalPredictDelta += float64(predict) - act

			avgPredictDeltaAvgFreq += math.Abs(float64(predictAvgFreq) - act)
			totalPredictDeltaAvgFreq += float64(predictAvgFreq) - act
		}

		avgPredictDelta /= float64(len(tscDeltas) - trainSetCnt)
	} else {
		for i := range tscs[trainSetCnt:] {
			predict := int64(tscs[i+trainSetCnt]*coeff) + offset
			predictAvgFreq := int64(tscs[i+trainSetCnt]*avgCoeff) + avgOffset

			act := syss[i+trainSetCnt]

			avgPredictDelta += math.Abs(float64(predict) - act)
			totalPredictDelta += float64(predict) - act

			avgPredictDeltaAvgFreq += math.Abs(float64(predictAvgFreq) - act)
			totalPredictDeltaAvgFreq += float64(predictAvgFreq) - act
		}

		avgPredictDelta /= float64(len(tscs) - trainSetCnt)
	}

	log.Printf("prediction made by %s and system clock, avg abs delta %.2fus, total non-abs dealta: %.2fus\n",
		simulateFuncName, avgPredictDelta/1000, totalPredictDelta/1000)

	avgPredictDeltaAvgFreq /= float64(len(tscDeltas))
	log.Printf("prediction made by avg frequency and system clock, avg abs delta %.2fus, total non-abs dealta: %.2fus\n",
		avgPredictDeltaAvgFreq/1000, totalPredictDeltaAvgFreq/1000)
}

// getClosestTSCSys tries to get the closest tsc register value nearby the system clock in a loop.
func getClosestTSCSys(n int) (int64, int64, int64) {
	// 256 is enough for finding the lowest sys clock cost in most cases.
	// Although time.Now() is using VDSO to get time, but it's unstable,
	// sometimes it will take more than 1000ns,
	// we have to use a big loop(e.g. 256) to get the "real" clock.
	// And it won't take a long time to finish calibrating job, only about 20µs.
	// [tscClock, wc, tscClock, wc, ..., tscClock]
	timeline := make([]int64, n+n+1)

	timeline[0] = tsc.GetInOrder()
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = time.Now().UnixNano()
		timeline[i+1] = tsc.GetInOrder()
	}

	// The minDelta is the smallest gap between two adjacent tscs,
	// which means the smallest gap between sys clock and tscClock too.
	minDelta := int64(math.MaxInt64)
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

	tscClock := (timeline[minIndex+1] + timeline[minIndex-1]) >> 1
	sys := timeline[minIndex]

	return minDelta, tscClock, sys
}

func isEven(n int) bool {
	return n&1 == 0
}

// simpleLinearRegression without intercept:
// α = ∑𝑥𝑖𝑦𝑖 / ∑𝑥𝑖^2.
func simpleLinearRegression(tscs, syss []float64) (float64, int64) {
	denominator, numerator := float64(0), float64(0)
	for i := range tscs {
		numerator += tscs[i] * syss[i]
		denominator += tscs[i] * tscs[i]
	}

	coeff := numerator / denominator

	return coeff, 0
}

func simpleLinearRegressionWithIntercept(tscs, syss []float64) (float64, int64) {
	tmean, wmean := float64(0), float64(0)
	for i := range tscs {
		tmean += tscs[i]
		wmean += syss[i]
	}

	tmean /= float64(len(tscs))
	wmean /= float64(len(syss))

	denominator, numerator := float64(0), float64(0)
	for i := range tscs {
		numerator += (tscs[i] - tmean) * (syss[i] - wmean)
		denominator += (tscs[i] - tmean) * (tscs[i] - tmean)
	}

	coeff := numerator / denominator

	return coeff, int64(wmean - coeff*tmean)
}

func nanosFmt(ns int64) string {
	return time.Unix(0, ns).Format(time.RFC3339Nano)
}
