// gofreq is a tool to print the better TSC frequency as a clock source.

package main

import (
	"fmt"
	"github.com/templexxx/cpu"
	"log"
	"time"

	"github.com/templexxx/tsc"
	"github.com/zaibyte/pkg/config"
)

type Config struct {
	JobTime int64 `toml:"job_time"` // Minutes.
}

const _appName = "gofreq"

func main() {
	config.Init(_appName)

	var cfg Config
	config.Load(&cfg)

	r := &runner{cfg: &cfg}
	r.run()
}

type runner struct {
	cfg *Config
}

func (r *runner) run() {

	if !tsc.Enabled {
		return
	}

	fmt.Printf("start at: %s\n", time.Now().Format(time.RFC3339Nano))

	r.doJobLoop()
}

func (r *runner) doJobLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	total := r.cfg.JobTime

	if total < 20 {
		log.Fatal("job time too short")
	}

	deltas := make([]float64, total) // It must be linear, keeping up or down.
	cnt := 0
	for {
		if cnt >= int(total) {
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

	deltas = deltas[5:]             // Drop the first 5.
	deltas = deltas[:len(deltas)-5] // Drop the last 5.

	var ddTotal float64 // deltas' deltas.
	for i := 1; i < len(deltas); i++ {
		ddTotal += deltas[i] - deltas[i-1]
	}

	dd := ddTotal / float64(len(deltas)-1)
	c := dd / (60 * 1000 * 1000 * 1000)

	freq := float64(cpu.X86.TSCFrequency) * (1 + c)

	fmt.Printf("cpu: %s_%d, new freq: %d, old: %d\n, avg_delta :%.2f, adjustment: %.2f",
		cpu.X86.Signature, cpu.X86.SteppingID, uint64(freq), cpu.X86.TSCFrequency, dd, c)
}
