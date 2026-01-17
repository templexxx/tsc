package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/templexxx/tsc"
)

func main() {
	calibrateInterval := 10 * time.Second
	threads := 3

	ctx, cancel := context.WithCancel(context.Background())

	if tsc.Supported() {
		go func(ctx context.Context) {
			log.Println("Start background calibrating")

			ctx2, cancel2 := context.WithCancel(ctx)
			defer cancel2()

			ticker := time.NewTicker(calibrateInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					tsc.Calibrate()
					log.Println("Calibration done")
				case <-ctx2.Done():
					return
				}
			}
		}(ctx)
	} else {
		log.Println("TSC not supported")
	}

	waitGroup := new(sync.WaitGroup)
	waitGroup.Add(threads)

	for i := range threads {
		go func(i int) {
			defer waitGroup.Done()

			for range 10 {
				systemClock := time.Now().UnixNano()
				tscClock := tsc.UnixNano()
				delta := float64(tscClock) - float64(systemClock)
				log.Printf("Thread %d, System Clock: %d TSC Clock: %d Delta: %.2f μs\n",
					i, systemClock, tscClock, delta/1000)
				time.Sleep(5 * time.Second)
			}
		}(i)
	}

	waitGroup.Wait()
	cancel()
}
