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
	sample   = flag.Int("sample", 100, "number of samples, the more samples the better result")
	duration = flag.Int64("duration", 250, "duration(ms) between two timestamp, we need a bit longer duration for make result useful in long term")
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
	tscs := make([]int64, cnt*2)
	walls := make([]int64, cnt*2)

	for j := 0; j < cnt; j++ { // Try to find the best one inside 64 tries (avoiding jitter).
		md0, tscc0, wallc0 := getClosestTSCWall(256)
		if md0 > minTSCDelta {
			log.Fatalf("the min tsc delta too big, exp <= %d, but got: %d", minTSCDelta, md0)
		}
		time.Sleep(du)
		md1, tscc1, wallc1 := getClosestTSCWall(256)
		if md1 > minTSCDelta {
			log.Fatalf("the min tsc delta too big, exp <= %d, but got: %d", minTSCDelta, md1)
		}

		if wallc1-wallc0 < int64(du) {
			log.Fatalf("wall clock goes backwards, exp %s, but got: %.2fms", du.String(), float64(wallc1-wallc0)/float64(time.Millisecond))
		}

		freq := (float64(tscc1-tscc0) / float64(wallc1-wallc0)) * 1e9
		freqs[j] = freq
		fmt.Printf("round: %d freq is: %.9f, min tsc deltas are: %d, %d, duration of closest wallclock: %.2fms\n",
			j, freq, md0, md1, float64(wallc1-wallc0)/1000/1000)

		tscs[j*2] = tscc0
		tscs[j*2+1] = tscc1

		walls[j*2] = wallc0
		walls[j*2+1] = wallc1
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

	c, offset := simpleLinearRegression(tscs, walls)
	fmt.Printf("result of simple linear regression, coeff: %.16f, offset: %d\n", c, offset)

	avgPredictDelta := float64(0)
	for i := range tscs {
		predict := int64(float64(tscs[i])*c) + offset
		act := walls[i]
		avgPredictDelta += math.Abs(float64(predict) - float64(act))
	}
	avgPredictDelta = avgPredictDelta / float64(len(tscs))
	fmt.Printf("avg abs delta of prediction made by simple linear regression and real clock: %.2fus\n", avgPredictDelta/1000)
}

// getClosestTSCWall tries to get the closest tsc register value nearby a certain clock in a loop.
func getClosestTSCWall(n int) (minDelta, tscClock, wall int64) {

	// 256 is enough for finding the lowest wall clock cost in most cases.
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
	// which means the smallest gap between wall clock and tscClock too.
	minDelta = int64(math.MaxInt64)
	minIndex := 1 // minIndex is wall clock index where has minDelta.

	// time.Now()'s precision is only µs (on macOS),
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

// class SimpleLinearRegression(object):
// """
// 简单线性回归方程，即一元线性回归,只有一个自变量，估值函数为: y = b0 + b1 * x
// """
// def __init__(self):
// self.b0 = 0
// self.b1 = 0
// def fit(self, x: list, y: list):
// n = len(x)
// x_mean = sum(x) / n
// y_mean = sum(y) / n
// dinominator = 0
// numerator = 0
// for xi, yi in zip(x, y):
// numerator += (xi - x_mean) * (yi - y_mean)
// dinominator += (xi - x_mean) ** 2
// self.b1 = numerator / dinominator
// self.b0 = y_mean - self.b1 * x_mean
//
// def pridict(self, x):
// return self.b0 + self.b1 * x
func simpleLinearRegression(tscs, walls []int64) (coeff float64, offset int64) {

	tmean, wmean := float64(0), float64(0)
	for i := range tscs {
		tmean += float64(tscs[i])
		wmean += float64(walls[i])
	}
	tmean = tmean / float64(len(tscs))
	wmean = wmean / float64(len(walls))

	denominator, numerator := float64(0), float64(0)
	for i := range tscs {
		numerator += (float64(tscs[i]) - tmean) * (float64(walls[i]) - wmean)
		denominator += math.Pow(float64(tscs[i])-tmean, 2)
	}

	coeff = numerator / denominator

	return coeff, int64(wmean - coeff*tmean)
}
