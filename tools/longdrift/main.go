// longdrift is a tool to print the delta between system clock & tsc.

package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"

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
	coeff             = flag.Float64("coeff", 0, "")
	offset            = flag.Int64("offset", 0, "")
	cmpsys            = flag.Bool("cmp_sys", false, "compare two system clock but not system clock and tsc clock")
	InOrder           = flag.Bool("in_order", false, "get tsc register in-order (with lfence)")
)

var compareFunc = tsc.UnixNano()

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

	if *cmpsys {
		compareFunc = time.Now().UnixNano()
	}
	if *InOrder {
		tsc.ForbidOutOfOrder()
	}

	cfg := Config{
		JobTime:           *jobTime,
		EnableCalibrate:   *enableCalibrate,
		CalibrateInterval: time.Duration(*calibrateInterval) * time.Second,
		Idle:              *idle,
		Print:             *printDelta,
		Threads:           *threads,
	}

	deltas := make([][]int64, cfg.Threads)
	for i := range deltas {
		deltas[i] = make([]int64, cfg.JobTime)
	}

	r := &runner{cfg: &cfg, deltas: deltas}

	r.run()
}

type runner struct {
	cfg    *Config
	deltas [][]int64
}

func (r *runner) run() {

	if !tsc.Supported() {
		fmt.Println("tsc unsupported")
		return
	}

	tsc.ForceTSC() // Enable TSC force.

	if *coeff != 0 && *offset != 0 {
		freq := 1e9 / *coeff
		r.cfg.TSCFreq = freq
		tsc.CalibrateWithCoeffOffset(*coeff, *offset)
		r.cfg.Source = "option"
	} else {
		freq := *tscFreq
		if freq != 0 {
			r.cfg.TSCFreq = freq
			tsc.SetFreq(freq)
			tsc.Calibrate() // Reset offset.
			r.cfg.Source = "option"
		} else {
			r.cfg.TSCFreq = tsc.Frequency
			r.cfg.Source = tsc.FreqSource
		}
	}

	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	fmt.Printf("cpu: %s, tsc_freq: %.9f, offset: %d, source: %s\n", cpuFlag, tsc.Frequency, tsc.Offset, r.cfg.Source)

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

	r.printDeltas()
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

	minDelta, minDeltaABS := int64(0), math.MaxFloat64
	maxDelta, maxDeltaABS := int64(0), float64(0)

	cmpTo := "tsc"
	if *cmpsys {
		cmpTo = "sys_clock2"
	}

	for i := 0; i < int(r.cfg.JobTime); i++ {

		time.Sleep(time.Second)
		clock2 := compareFunc
		clock1 := time.Now().UnixNano()
		delta := clock2 - clock1
		r.deltas[thread][i] = delta

		deltaABS := math.Abs(float64(delta))
		if deltaABS < minDeltaABS {
			minDeltaABS = deltaABS
			minDelta = delta
		}
		if deltaABS > maxDeltaABS {
			maxDeltaABS = deltaABS
			maxDelta = delta
		}

		if r.cfg.Print {
			fmt.Printf("thread: %d, sys_clock: %d, %s: %d, delta: %.2fus\n",
				thread, clock1, cmpTo, clock2, float64(delta)/float64(time.Microsecond))
		}
	}
	fmt.Printf("thread: %d, first_delta: %.2fus, last_delta: %.2fus, min_delta: %.2fus, max_delta: %.2fus\n",
		thread,
		float64(r.deltas[thread][0])/float64(time.Microsecond),
		float64(r.deltas[thread][r.cfg.JobTime-1])/float64(time.Microsecond),
		float64(minDelta)/float64(time.Microsecond),
		float64(maxDelta)/float64(time.Microsecond))
}

func (r *runner) printDeltas() {

	p := plot.New()

	cmpTo := "TSC"
	if *cmpsys {
		cmpTo = "Syc Clock2"
	}

	p.Title.Text = fmt.Sprintf("%s - Sys Clock", cmpTo)
	p.X.Label.Text = "Time(s)"
	p.Y.Label.Text = "Delta(us)"

	for i := range r.deltas {
		err := plotutil.AddLinePoints(p,
			fmt.Sprintf("thread: %d", i),
			makePoints(r.deltas[i]))
		if err != nil {
			panic(err)
		}
	}

	if err := p.Save(10*vg.Inch, 10*vg.Inch, fmt.Sprintf("longdrift_%s.PNG", time.Now().Format(time.RFC3339))); err != nil {
		panic(err)
	}
}

func makePoints(deltas []int64) plotter.XYs {
	points := make(plotter.XYs, len(deltas))
	for i := range points {
		points[i].X = float64(i) + 1
		points[i].Y = float64(deltas[i]) / float64(time.Microsecond)
	}

	return points
}
