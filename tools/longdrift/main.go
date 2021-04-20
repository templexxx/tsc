// longdrift is a tool to print the delta between system wall clock & tsc.

package main

import (
	"context"
	"fmt"
	"github.com/klauspost/cpuid/v2"
	"github.com/templexxx/cpu"
	"runtime"
	"time"

	"github.com/templexxx/tsc"
	"github.com/zaibyte/pkg/config"
)

type Config struct {
	JobTime           int64 `toml:"job_time"` // Seconds.
	Interval          int64 `toml:"interval"` // Seconds.
	EnableCalibrate   bool  `toml:"enable_calibrate"`
	CalibrateInterval int64 `toml:"calibrate_interval"`
	Idle              bool  `toml:"idle"`
}

const _appName = "longdrift"

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
		fmt.Println("tsc unsupported")
		return
	}

	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	fmt.Printf("cpu: %s, tsc_freq: %.2f\n", cpuFlag, tsc.FreqTbl[cpuFlag])

	ctx, cancel := context.WithCancel(context.Background())

	if r.cfg.EnableCalibrate {
		go func(ctx context.Context) {

			ctx2, cancel2 := context.WithCancel(ctx)
			defer cancel2()

			ticker := time.NewTicker(time.Duration(r.cfg.CalibrateInterval) * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					tsc.Calibrate()
				case <-ctx2.Done():
					break
				}
			}
		}(ctx)
	}

	go takeCPU(ctx, r.cfg.Idle)

	r.doJobLoop()
	cancel()
}

func takeCPU(ctx context.Context, idle bool) {

	if idle {
		return
	}

	cnt := runtime.NumCPU()

	hz := cpuid.CPU.Hz
	if hz == 0 {
		hz = 3 * 1000 * 1000 * 1000 // Assume 3GHz.
	}

	for i := 0; i < cnt; i++ {
		go func(ctx context.Context) {
			ctx2, cancel := context.WithCancel(ctx)
			defer cancel()

			for {
				select {
				case <-ctx2.Done():
					return
				default:
				}

				// Empty loop may cost about 5 uops.
				for j := 0; j < int(hz/5); j++ {
				}
				time.Sleep(time.Second)
			}

		}(ctx)
	}
}

func (r *runner) doJobLoop() {
	ticker := time.NewTicker(time.Duration(r.cfg.Interval) * time.Second)
	defer ticker.Stop()

	end := time.Now().Add(time.Duration(r.cfg.JobTime) * time.Second)

	for {
		if time.Now().After(end) {
			return
		}
		<-ticker.C
		tscT := tsc.UnixNano()
		wall := time.Now().UnixNano()
		fmt.Printf("wall_clock: %d, tsc: %d, delta: %dus\n",
			wall, tscT, (tscT-wall)/int64(time.Microsecond))
	}
}
