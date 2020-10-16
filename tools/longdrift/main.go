// longdrift is a tool to print the delta between system wall clock & tsc.

package main

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/templexxx/tsc"
	"github.com/zaibyte/pkg/config"
)

type Config struct {
	JobTime           int64 `toml:"job_time"` // Seconds.
	Interval          int64 `toml:"interval"` // Seconds.
	EnableCalibrate   bool  `toml:"enable_calibrate"`
	CalibrateInterval int64 `toml:"calibrate_interval"`
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

	fmt.Printf("coeff: %.6f\n", math.Float64frombits(atomic.LoadUint64(&tsc.Coeff)))

	if r.cfg.EnableCalibrate {
		go func() {
			ticker := time.NewTicker(time.Duration(r.cfg.CalibrateInterval) * time.Second)
			defer ticker.Stop()

			for {
				<-ticker.C
				tsc.Calibrate()
			}
		}()
	}

	r.doJobLoop()
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
		//wall := time.Now()
		wall := time.Now().UnixNano()
		//fmt.Printf("%s, wall_clock: %d, tsc: %d, delta: %d\n",
		//	wall.Format(time.RFC3339Nano), wall.UnixNano(), tscT, tscT-wall.UnixNano())
		fmt.Printf("wall_clock: %d, tsc: %d, delta: %d\n",
			wall, tscT, tscT-wall)
	}
}
