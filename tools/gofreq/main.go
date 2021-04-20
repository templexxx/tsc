// gofreq is a tool to print the better TSC frequency as a clock source.

package main

import (
	"flag"
	"fmt"
	"github.com/templexxx/cpu"
	"log"
	"math"
	"sort"
	"sync/atomic"
	"time"

	"github.com/templexxx/tsc"
)

var (
	JobTime = flag.Int("job_time", 1, "job time, minute")
	Round   = flag.Int("round", 10, "run job round and round")
)

func main() {

	flag.Parse()

	r := &runner{jobTime: time.Duration(int64(*JobTime)) * time.Minute, round: *Round}
	r.run()
}

type runner struct {
	jobTime time.Duration
	round   int
}

func (r *runner) run() {

	if !tsc.Enabled {
		fmt.Println("tsc unsupported")
		return
	}

	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	fmt.Printf("cpu: %s start at: %s\n", cpuFlag, time.Now().Format(time.RFC3339Nano))

	for i := 0; i < r.round; i++ {
		newFreq, min, max, avgDelta := r.doJobLoop()
		freq := 1e9 / math.Float64frombits(atomic.LoadUint64(&tsc.Coeff))
		fmt.Printf("round: %d, freq: %.2f, avg_delta: %.2fns, min_delta: %.2fns, max_delta: %.2fns\n",
			i, freq, avgDelta, min, max)
		atomic.StoreUint64(&tsc.Coeff, math.Float64bits(1/(newFreq/1e9)))
	}
}

func (r *runner) doJobLoop() (newFreq float64, min, max, avgDelta float64) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	jobTime := r.jobTime

	if jobTime < time.Minute {
		log.Fatal("job time too short, at least 20mins")
	}

	deltas := make([]float64, (jobTime/time.Minute)*60*100) // Every 10ms get a timestamp.
	cnt := 0
	for {
		if cnt >= len(deltas) {
			break
		}

		<-ticker.C
		// Get time by tsc clock first, because wall clock may take too much time,
		// it will impact the result of tsc clock.
		tscT := tsc.UnixNano()
		wall := time.Now().UnixNano()
		delta := tscT - wall
		deltas[cnt] = float64(delta)
		cnt++
	}

	deltasCp := make([]float64, len(deltas))
	copy(deltasCp, deltas)

	sort.Float64s(deltasCp)

	mins := deltasCp[:256]
	maxs := deltasCp[len(deltasCp)-256:]

	var ddTotal float64 // deltas' deltas.
	for i := 1; i < len(deltas); i++ {
		if isIn(deltas[i], mins) || isIn(deltas[i], maxs) {
			continue
		}
		ddTotal += deltas[i] - deltas[i-1]
	}

	dd := ddTotal / (float64(len(deltas)-1) - 512)
	c := -dd / (10 * 1000 * 1000)

	newFreq = float64(cpu.X86.TSCFrequency) * (1 + c)

	totalDelta := float64(0)
	for i := range deltas {
		totalDelta += deltas[i]
	}

	return newFreq, mins[0], maxs[255], totalDelta / float64(len(deltas))
}

func isIn(f float64, fs []float64) bool {
	for _, v := range fs {
		if f == v {
			return true
		}
	}
	return false
}
