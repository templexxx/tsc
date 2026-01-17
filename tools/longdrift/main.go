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

	"github.com/klauspost/cpuid/v2"
	"github.com/templexxx/cpu"
	"github.com/templexxx/tsc"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

type Config struct {
	JobTime           int64
	EnableCalibrate   bool
	CalibrateInterval time.Duration
	Idle              bool
	Print             bool
	Threads           int
	Coeff             float64
	CmpSys            bool
	InOrder           bool
}

func sysClock() int64 {
	return time.Now().UnixNano()
}

func tscClock() int64 {
	return tsc.UnixNano()
}

func main() {
	jobTimeFlag := flag.Int64("job_time", 1200, "unit: seconds")
	enableCalibrateFlag := flag.Bool("enable_calibrate", false, "enable calibrate will help to catch up system clock")
	calibrateIntervalFlag := flag.Int64("calibrate_interval", 300, "unit: seconds")
	idleFlag := flag.Bool("idle", true, "if false it will run empty loops on each cores, try to simulate a busy cpu")
	printDetailsFlag := flag.Bool("print", false, "print every second delta & calibrate result")
	threadsFlag := flag.Int("threads", 1, "try to run comparing on multi cores")
	coeffFlag := flag.Float64("coeff", 0, "coefficient for tsc: tsc_register * coeff + offset = timestamp")
	cmpsysFlag := flag.Bool("cmp_sys", false, "compare two system clock")
	inOrderFlag := flag.Bool("in_order", false, "get tsc register in-order (with lfence)")

	flag.Parse()

	var cmpClock func() int64
	if *cmpsysFlag {
		cmpClock = sysClock
	} else {
		cmpClock = tscClock
	}

	if *inOrderFlag {
		tsc.ForbidOutOfOrder()
	}

	cfg := Config{
		JobTime:           *jobTimeFlag,
		EnableCalibrate:   *enableCalibrateFlag,
		CalibrateInterval: time.Duration(*calibrateIntervalFlag) * time.Second,
		Idle:              *idleFlag,
		Print:             *printDetailsFlag,
		Threads:           *threadsFlag,
		Coeff:             *coeffFlag,
		CmpSys:            *cmpsysFlag,
		InOrder:           *inOrderFlag,
	}

	deltas := make([][]int64, cfg.Threads)
	for i := range deltas {
		deltas[i] = make([]int64, cfg.JobTime)
	}

	r := &runner{cfg: &cfg, deltas: deltas, wg: nil, cmpClock: cmpClock}

	r.run()
}

type runner struct {
	cfg    *Config
	deltas [][]int64

	wg       *sync.WaitGroup
	cmpClock func() int64
}

func (r *runner) run() {
	if !tsc.Supported() {
		log.Fatal("tsc unsupported")
	}

	start := time.Now()
	log.Printf("job start at: %s\n", start.Format(time.RFC3339Nano))

	if r.cfg.Coeff != 0 {
		tsc.CalibrateWithCoeff(r.cfg.Coeff)
	}

	options := ""

	flag.VisitAll(func(f *flag.Flag) {
		options += fmt.Sprintf(" -%s %s", f.Name, f.Value)
	})

	log.Printf("testing with options:%s\n", options)

	cpuFlag := fmt.Sprintf("%s_%d", cpu.X86.Signature, cpu.X86.SteppingID)

	ooffset, ocoeff := tsc.LoadOffsetCoeff(tsc.OffsetCoeffAddr)

	log.Printf("cpu: %s, begin with tsc_freq: %.16f(coeff: %.16f), offset: %d\n", cpuFlag, 1e9/ocoeff, ocoeff, ooffset)

	ctx, cancel := context.WithCancel(context.Background())

	if r.cfg.EnableCalibrate {
		go r.backgroundCalibrate(ctx)
	}

	go takeCPU(ctx, r.cfg.Idle)

	waitGroup := new(sync.WaitGroup)
	waitGroup.Add(r.cfg.Threads)
	r.wg = waitGroup

	for i := range r.cfg.Threads {
		go func(i int) {
			r.doJobLoop(i)
		}(i)
	}

	waitGroup.Wait()
	cancel()

	cost := time.Since(start)
	log.Printf("job taken: %s\n", cost.String())

	r.printDeltas()
}

func (r *runner) backgroundCalibrate(ctx context.Context) {
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
			if r.cfg.Print {
				log.Printf("origin tsc_freq: %.16f, new_tsc_freq: %.16f\n", originFreq, 1e9/ocoeff)
			}
		case <-ctx2.Done():
			return
		}
	}
}

func takeCPU(ctx context.Context, idle bool) {
	if idle {
		return
	}

	cnt := runtime.NumCPU()

	freq := cpuid.CPU.Hz
	if freq == 0 {
		freq = 3 * 1000 * 1000 * 1000 // Assume 3GHz.
	}

	for range cnt {
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
				for range freq / 5 {
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
	if r.cfg.CmpSys {
		cmpTo = "sys_clock2"
	}

	for i := range r.cfg.JobTime {
		time.Sleep(time.Second)

		clock2 := r.cmpClock()
		sysClock := time.Now().UnixNano()
		clock22 := r.cmpClock()
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
			log.Printf("thread: %d, sys_clock: %d, %s: %d, delta: %.2fus, next_delta: %.2fus\n",
				thread, sysClock, cmpTo, clock2,
				float64(delta)/float64(time.Microsecond),
				float64(delta2)/float64(time.Microsecond))
		}
	}

	totalDelta := float64(0)
	for _, delta := range r.deltas[thread] {
		totalDelta += math.Abs(float64(delta))
	}

	avgDelta := totalDelta / float64(r.cfg.JobTime)

	log.Printf("[thread-%d] delta(abs): first: %.2fus, last: %.2fus, min: %.2fus, max: %.2fus, mean: %.2fus\n",
		thread,
		math.Abs(float64(r.deltas[thread][0])/float64(time.Microsecond)),
		math.Abs(float64(r.deltas[thread][r.cfg.JobTime-1])/float64(time.Microsecond)),
		math.Abs(float64(minDelta)/float64(time.Microsecond)),
		math.Abs(float64(maxDelta)/float64(time.Microsecond)),
		avgDelta/1000)
}

func (r *runner) printDeltas() {
	plot := plot.New()

	cmpTo := "TSC"
	if r.cfg.CmpSys {
		cmpTo = "Syc Clock2"
	}

	plot.Title.Text = cmpTo + " - Sys Clock"
	plot.X.Label.Text = "Time(s)"
	plot.Y.Label.Text = "Delta(us)"

	for i := range r.deltas {
		err := plotutil.AddLinePoints(plot,
			fmt.Sprintf("thread: %d", i),
			makePoints(r.deltas[i]))
		if err != nil {
			panic(err)
		}
	}

	const outTmFmt = "2006-01-02T150405"

	err := plot.Save(10*vg.Inch, 10*vg.Inch, fmt.Sprintf("longdrift_%s.PNG", time.Now().Format(outTmFmt)))
	if err != nil {
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
