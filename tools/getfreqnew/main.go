package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/templexxx/cpu"
	"github.com/templexxx/tsc"
)

var (
	sample   = flag.Int("sample", 10, "number of samples")
	duration = flag.Int64("duration", 1000, "duration(ms) between two timestamp, tsc_ freq = (tsc_b - tsc_a) / (wall_clock_b - wall_clock_a)")
)

const sleepDuration = time.Second

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

	du := time.Duration(*duration) * time.Millisecond
	if du == 0 {
		du = sleepDuration
	}

	start := time.Now()

	freqs := make([]float64, cnt)

	for j := 0; j < cnt; j++ { // Try to find the best one inside 64 tries (avoiding jitter).
		md0, tscc0, wallc0 := getClosestTSCWall(256)
		if md0 > 800 {
			log.Fatalf("the min tsc delta too big, exp <= 800, but got: %d", md0)
		}
		time.Sleep(sleepDuration)
		md1, tscc1, wallc1 := getClosestTSCWall(256)
		if md1 > 800 {
			log.Fatalf("the min tsc delta too big, exp <= 800, but got: %d", md1)
		}

		if wallc1-wallc0 < int64(sleepDuration) {
			log.Fatalf("wall clock goes backwards, exp %s, but got: %.2fms", sleepDuration.String(), float64(wallc1-wallc0)/float64(time.Millisecond))
		}

		freq := (float64(tscc1-tscc0) / float64(wallc1-wallc0)) * 1e9
		freqs[j] = freq
		fmt.Printf("round: %d freq is: %.9f, min tsc deltas are: %d, %d\n", j, freq, md0, md1)
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
