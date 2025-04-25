// longdrift is a tool to print the delta between system clock & tsc.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	jobTime           = flag.Int64("job_time", 1200, "unit: seconds")
	enableCalibrate   = flag.Bool("enable_calibrate", false, "enable calibrate will help to catch up system clock")
	calibrateInterval = flag.Int64("calibrate_interval", 300, "unit: seconds")
	idle              = flag.Bool("idle", true, "if false it will run empty loops on each cores, try to simulate a busy cpu")
	printDetails      = flag.Bool("print", false, "print every second delta & calibrate result")
	threads           = flag.Int("threads", 1, "try to run comparing on multi cores")
	coeff             = flag.Float64("coeff", 0, "coefficient for tsc: tsc_register * coeff + offset = timestamp")
	cmpsys            = flag.Bool("cmp_sys", false, "compare two system clock")
	inOrder           = flag.Bool("in_order", false, "get tsc register in-order (with lfence)")
)

type Config struct {
	JobTime           int64
	EnableCalibrate   bool
	CalibrateInterval time.Duration
	Idle              bool
	Print             bool
	Threads           int
	Coeff             float64
}

var cmpClock func() int64

func sysClock() int64 {
	return time.Now().UnixNano()
}

func tscClock() int64 {
	return tsc.UnixNano()
}

func main() {

	flag.Parse()

	if *cmpsys {
		cmpClock = sysClock
	} else {
		cmpClock = tscClock
	}

	if *inOrder {
		tsc.ForbidOutOfOrder()
	}

	cfg := Config{
		JobTime:           *jobTime,
		EnableCalibrate:   *enableCalibrate,
		CalibrateInterval: time.Duration(*calibrateInterval) * time.Second,
		Idle:              *idle,
		Print:             *printDetails,
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

	wg *sync.WaitGroup
}

func (r *runner) run() {

	if !tsc.Supported() {
		log.Fatal("tsc unsupported")
	}

	start := time.Now()
	fmt.Printf("job start at: %s\n", start.Format(time.RFC3339Nano))

	if *coeff != 0 {
		tsc.CalibrateWithCoeff(*coeff)
	}

	options := ""
	flag.VisitAll(func(f *flag.Flag) {
		options += fmt.Sprintf(" -%s %s", f.Name, f.Value)
	})

	fmt.Printf("testing with options:%s\n", options)
	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	ooffset, ocoeff := tsc.LoadOffsetCoeff(tsc.OffsetCoeffAddr)

	fmt.Printf("cpu: %s, begin with tsc_freq: %.16f(coeff: %.16f), offset: %d\n", cpuFlag, 1e9/ocoeff, ocoeff, ooffset)

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
					_, ocoeff := tsc.LoadOffsetCoeff(tsc.OffsetCoeffAddr)

					originFreq := 1e9 / ocoeff
					tsc.Calibrate()
					_, ocoeff = tsc.LoadOffsetCoeff(tsc.OffsetCoeffAddr)
					if *printDetails {
						fmt.Printf("origin tsc_freq: %.16f, new_tsc_freq: %.16f\n", originFreq, 1e9/ocoeff)
					}
				case <-ctx2.Done():
					break
				}
			}
		}(ctx)
	}

	go takeCPU(ctx, r.cfg.Idle)

	wg := new(sync.WaitGroup)
	wg.Add(r.cfg.Threads)
	r.wg = wg

	for i := 0; i < r.cfg.Threads; i++ {
		go func(i int) {
			r.doJobLoop(i)
		}(i)
	}
	wg.Wait()
	cancel()

	cost := time.Now().Sub(start)
	fmt.Printf("job taken: %s\n", cost.String())

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

func (r *runner) doJobLoop(thread int) {
	defer r.wg.Done()

	minDelta, minDeltaABS := int64(0), math.MaxFloat64
	maxDelta, maxDeltaABS := int64(0), float64(0)

	cmpTo := "tsc"
	if *cmpsys {
		cmpTo = "sys_clock2"
	}

	for i := 0; i < int(r.cfg.JobTime); i++ {

		time.Sleep(time.Second)
		clock2 := cmpClock()
		sysClock := time.Now().UnixNano()
		clock22 := cmpClock()
		delta := (clock2+clock22)/2 - sysClock
		delta2 := clock22 - sysClock
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
			fmt.Printf("thread: %d, sys_clock: %d, %s: %d, delta: %.2fus, next_delta: %.2fus\n",
				thread, sysClock, cmpTo, clock2, float64(delta)/float64(time.Microsecond), float64(delta2)/float64(time.Microsecond))
		}
	}

	totalDelta := float64(0)
	for _, delta := range r.deltas[thread] {
		totalDelta += math.Abs(float64(delta))
	}
	avgDelta := totalDelta / float64(r.cfg.JobTime)

	fmt.Printf("[thread-%d] delta(abs): first: %.2fus, last: %.2fus, min: %.2fus, max: %.2fus, mean: %.2fus\n",
		thread,
		math.Abs(float64(r.deltas[thread][0])/float64(time.Microsecond)),
		math.Abs(float64(r.deltas[thread][r.cfg.JobTime-1])/float64(time.Microsecond)),
		math.Abs(float64(minDelta)/float64(time.Microsecond)),
		math.Abs(float64(maxDelta)/float64(time.Microsecond)),
		avgDelta/1000)
}

var outTmFmt = "2006-01-02T150405"

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

	if err := p.Save(10*vg.Inch, 10*vg.Inch, fmt.Sprintf("longdrift_%s.PNG", time.Now().Format(outTmFmt))); err != nil {
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
