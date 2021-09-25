package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/templexxx/cpu"
	"github.com/templexxx/tsc"
)

var (
	sample      = flag.Int("sample", 1024, "number of samples, the more samples the better result")
	duration    = flag.Int64("duration", 10, "duration(ms) between two timestamp, we need a bit longer duration for make result useful in long term")
	printSample = flag.Bool("print", false, "print every sample")
)

const (
	sleepDuration          = time.Second
	minTSCDeltaMac   int64 = 800
	minTSCDeltaLinux int64 = 800
)

func main() {
	flag.Parse()

	if !tsc.Supported() {
		fmt.Println("tsc unsupported")
		return
	}

	tsc.ForceTSC() // Enable TSC force.

	cnt := *sample

	if cnt < 2 {
		cnt = 2
	}

	minTSCDelta := minTSCDeltaLinux
	if runtime.GOOS == `darwin` {
		minTSCDelta = minTSCDeltaMac
	}

	du := time.Duration(*duration) * time.Millisecond
	if du == 0 {
		du = sleepDuration
	}

	start := time.Now()

	freqs := make([]float64, cnt)
	tscDeltas := make([]float64, cnt)
	sysDeltas := make([]float64, cnt)
	tscs := make([]int64, cnt*2)
	syss := make([]int64, cnt*2)

	minMD := int64(math.MaxInt64)
	minTSC, minSys := int64(0), int64(0)

	for j := 0; j < cnt; j++ { // Try to find the best one inside 64 tries (avoiding jitter).
		md0, tscc0, sys0 := getClosestTSCSys(256)
		if md0 > minTSCDelta {
			log.Fatalf("the min tsc delta too big, exp <= %d, but got: %d", minTSCDelta, md0)
		}

		time.Sleep(du)
		md1, tscc1, sys1 := getClosestTSCSys(256)
		if md1 > minTSCDelta {
			log.Fatalf("the min tsc delta too big, exp <= %d, but got: %d", minTSCDelta, md1)
		}

		if sys1-sys0 < int64(du) {
			log.Fatalf("sys clock goes backwards, exp %s, but got: %.2fms", du.String(), float64(sys1-sys0)/float64(time.Millisecond))
		}

		freq := (float64(tscc1-tscc0) / float64(sys1-sys0)) * 1e9
		freqs[j] = freq

		if *printSample {
			fmt.Printf("round: %d freq is: %.9f, min tsc deltas are: %d, %d, duration of closest sysclock: %.2fms\n",
				j, freq, md0, md1, float64(sys1-sys0)/1000/1000)
		}

		tscDeltas[j] = float64(tscc1 - tscc0)
		sysDeltas[j] = float64(sys1 - sys0)

		if md0 < minMD {
			minMD = md0
			minTSC = tscc0
			minSys = sys0
		}
		if md1 < minMD {
			minMD = md1
			minTSC = tscc1
			minSys = sys1
		}

		tscs[j*2] = tscc0
		tscs[j*2+1] = tscc1

		syss[j*2] = sys0
		syss[j*2+1] = sys1
	}

	cost := time.Now().Sub(start)

	avgFreq := float64(0)
	for _, f := range freqs {
		avgFreq += f
	}
	avgFreq = avgFreq / float64(cnt)

	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	fmt.Printf("cpu: %s, job cost: %.2fs\n", cpuFlag, cost.Seconds())
	fmt.Println("-------")
	fmt.Printf("origin freq is: %.9f\n", tsc.Frequency)
	fmt.Printf("avg freq is: %.9f\n", avgFreq)

	c := simpleLinearRegression(tscDeltas, sysDeltas)

	offset := minSys - int64(float64(minTSC)*c)

	fmt.Printf("result of simple linear regression, coeff: %.16f, freq: %.16f, offset: %d\n", c, 1e9/c, offset)

	avgPredictDelta := float64(0)
	totalPredictDelta := float64(0)
	for i := range tscDeltas {
		predict := int64(float64(tscDeltas[i]) * c)
		act := sysDeltas[i]
		avgPredictDelta += math.Abs(float64(predict) - act)
		totalPredictDelta += float64(predict) - act
	}
	avgPredictDelta = avgPredictDelta / float64(len(tscDeltas))
	fmt.Printf("prediction made by simple linear regression and real clock, avg abs delta %.2fus, total non-abs dealta: %.2fus\n",
		avgPredictDelta/1000, totalPredictDelta/1000)
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

	timeline[0] = tsc.GetInOrder()
	for i := 1; i < len(timeline)-1; i += 2 {
		timeline[i] = time.Now().UnixNano()
		timeline[i+1] = tsc.GetInOrder()
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

func isEven(n int) bool {
	return n&1 == 0
}

// simpleLinearRegression without intercept:
// α = ∑𝑥𝑖𝑦𝑖 / ∑𝑥𝑖^2.
func simpleLinearRegression(tscs, syss []float64) (coeff float64) {

	denominator, numerator := float64(0), float64(0)
	for i := range tscs {
		numerator += tscs[i] * syss[i]
		denominator += math.Pow(tscs[i], 2)
	}

	coeff = numerator / denominator

	return coeff
}
