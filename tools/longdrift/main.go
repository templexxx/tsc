// longdrift is a tool to print the delta between system wall clock & tsc.

package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elastic/go-hdrhistogram"
	"github.com/klauspost/cpuid/v2"
	"github.com/templexxx/cpu"
	"github.com/templexxx/tsc"
)

var (
	jobTime           = flag.Int64("job_time", 600, "unit: seconds")
	enableCalibrate   = flag.Bool("enable_calibrate", false, "enable calibrate will help to catch up system clock")
	calibrateInterval = flag.Int64("calibrate_interval", 30, "unit: seconds")
	idle              = flag.Bool("idle", true, "if false it will run empty loops on each cores, try to simulate a busy cpu")
	printDelta        = flag.Bool("print", false, "print every second delta")
	threads           = flag.Int("threads", 1, "try to run comparing on multi cores")
	tscFreq           = flag.Float64("freq", 0, "tsc frequency")
)

type Config struct {
	JobTime           int64
	EnableCalibrate   bool
	CalibrateInterval time.Duration
	Idle              bool
	Print             bool
	Threads           int
	TSCFreq           float64
	Source            string
}

func main() {

	flag.Parse()

	cfg := Config{
		JobTime:           *jobTime,
		EnableCalibrate:   *enableCalibrate,
		CalibrateInterval: time.Duration(*calibrateInterval) * time.Second,
		Idle:              *idle,
		Print:             *printDelta,
		Threads:           *threads,
	}

	r := &runner{cfg: &cfg}
	r.run()
}

type runner struct {
	cfg   *Config
	delta *hdrhistogram.Histogram
}

func (r *runner) run() {

	if !tsc.Enabled() {
		fmt.Println("tsc not enabled")
		return
	}

	freq := *tscFreq
	if freq != 0 {
		r.cfg.TSCFreq = freq
		r.cfg.Source = "option"
	} else {
		r.cfg.TSCFreq = 1e9 / math.Float64frombits(atomic.LoadUint64(&tsc.Coeff))
		r.cfg.Source = tsc.FreqSource
	}

	r.delta = hdrhistogram.New(-time.Second.Nanoseconds(), time.Second.Nanoseconds(), 3)

	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	fmt.Printf("cpu: %s, tsc_freq: %.9f, source: %s\n", cpuFlag, r.cfg.TSCFreq, r.cfg.Source)

	ctx, cancel := context.WithCancel(context.Background())

	if r.cfg.EnableCalibrate {
		go func(ctx context.Context) {

			ctx2, cancel2 := context.WithCancel(ctx)
			defer cancel2()

			ticker := time.NewTicker(r.cfg.CalibrateInterval)
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

	wg := new(sync.WaitGroup)
	wg.Add(r.cfg.Threads)

	for i := 0; i < r.cfg.Threads; i++ {
		go r.doJobLoop(i, wg)
	}
	wg.Wait()
	cancel()

	printLat("tsc-wall_clock", r.delta)
}

func printLat(name string, lats *hdrhistogram.Histogram) {
	fmt.Println(fmt.Sprintf("%s min: %d, avg: %.2f, max: %d",
		name, lats.Min(), lats.Mean(), lats.Max()))
	fmt.Println("delta(abs) percentiles (nsec):")
	fmt.Print(fmt.Sprintf(
		"|  1.00th=[%d],  5.00th=[%d], 10.00th=[%d], 20.00th=[%d],\n"+
			"| 30.00th=[%d], 40.00th=[%d], 50.00th=[%d], 60.00th=[%d],\n"+
			"| 70.00th=[%d], 80.00th=[%d], 90.00th=[%d], 95.00th=[%d],\n"+
			"| 99.00th=[%d], 99.50th=[%d], 99.90th=[%d], 99.95th=[%d],\n"+
			"| 99.99th=[%d]\n",
		lats.ValueAtQuantile(1), lats.ValueAtQuantile(5), lats.ValueAtQuantile(10), lats.ValueAtQuantile(20),
		lats.ValueAtQuantile(30), lats.ValueAtQuantile(40), lats.ValueAtQuantile(50), lats.ValueAtQuantile(60),
		lats.ValueAtQuantile(70), lats.ValueAtQuantile(80), lats.ValueAtQuantile(90), lats.ValueAtQuantile(95),
		lats.ValueAtQuantile(99), lats.ValueAtQuantile(99.5), lats.ValueAtQuantile(99.9), lats.ValueAtQuantile(99.95),
		lats.ValueAtQuantile(99.99)))
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

func (r *runner) doJobLoop(thread int, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	end := time.Now().Add(time.Duration(r.cfg.JobTime) * time.Second)

	cnt := 0
	delta, first, last := int64(0), int64(0), int64(0)
	for {
		if time.Now().After(end) {
			last = delta
			break
		}
		<-ticker.C
		tscT := tsc.UnixNano()
		wall := time.Now().UnixNano()
		delta = tscT - wall
		_ = r.delta.RecordValueAtomic(int64(math.Abs(float64(delta))))
		if r.cfg.Print {
			fmt.Printf("wall_clock: %d, tsc: %d, delta: %.2fus\n",
				wall, tscT, float64(delta)/float64(time.Microsecond))
		}
		if cnt == 0 {
			first = delta
		}
		cnt++
	}
	fmt.Printf("thread: %d, first_delta: %.2fus, last_delta: %.2fus\n",
		thread,
		float64(first)/float64(time.Microsecond),
		float64(last)/float64(time.Microsecond))
}
