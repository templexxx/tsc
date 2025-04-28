package main

import (
	"context"
	"fmt"
	"github.com/templexxx/tsc"
	"sync"
	"time"
)

func main() {

	calibrateInterval := 10 * time.Second
	threads := 3

	ctx, cancel := context.WithCancel(context.Background())

	if tsc.Supported() {
		go func(ctx context.Context) {

			fmt.Println("Start background calibrating")

			ctx2, cancel2 := context.WithCancel(ctx)
			defer cancel2()

			ticker := time.NewTicker(calibrateInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					tsc.Calibrate()
					fmt.Println("Calibration done")
				case <-ctx2.Done():
					return
				}
			}
		}(ctx)
	} else {
		fmt.Println("TSC not supported")
	}

	wg := new(sync.WaitGroup)
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func(i int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				systemClock := time.Now().UnixNano()
				tscClock := tsc.UnixNano()
				delta := float64(tscClock) - float64(systemClock)
				fmt.Printf("Thread %d, System Clock: %d TSC Clock: %d Delta: %.2f μs\n",
					i, systemClock, tscClock, delta/1000)
				time.Sleep(5 * time.Second)
			}
		}(i)
	}
	wg.Wait()
	cancel()
}
