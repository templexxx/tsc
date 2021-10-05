TSC
===

Get unix time (nanoseconds) in blazing low latency with high precision. About 6x~10x faster than time.Now().UnixNano(). 

## Could be used in ...

1. High performance log: as timestamp field
2. Benchmark: as time measurement
3. Just for fun: I have a cool clock.

It's not a good idea to use this lib for replacing system clock everywhere, before using it please read this documents carefully.

### Example1: Low latency trading

We want everything is fast in stock trading program (especially the speed of making money :D). Before marketing opens, we use NTP to force update machine's system clock, then the program run with this lib
will get a good starting, the drift will be under control well if the frequency used by system is good enough.

In this case, we could allow out-of-order for higher speed. Out-of-order could be ~10ns faster than in order version, it's important for trading function which total
cost is within hundreds nanoseconds. Out-of-order may bring dozens nanoseconds jitter, not big deal for this case because
what we want is just a faster timestamp for logging something.

### Example2: Measure only one function cost

For this case, we need in-order version for avoiding unexpected instructions going inside the function which we want to measure the speed.

## Compare to System Clock

1. Both of this lib & kernel are using TSC register if it's reliable. With Invariant TSC supports, we could get a reliable frequency even cross multi cores/CPUs.
2. Much faster than system clock, under 10ns for each invoking.
3. The cost of invoking is stable. Although time.Now() is using vDSO to get time, but it's not that stable either. 
In Go, we need to switch to g0 then switch back for [vDSO need more stack space](https://github.com/golang/go/issues/20427) which
makes things even "worse".
4. This lib could be calibrated according to wall clock periodically. But in this duration, we may be far away from system clock because the adjustment made by NTP. More details: `func Calibrate()` in [tsc_amd64.go](tsc_amd64.go)
5. Kernel adjusts tsc frequency by `adjtimex` too, `Calibrate()` will try best to catch up the changes made by kernel.
6. Kernel doesn't like float, the precision of clock isn't as good as this one.
7. This lib has a better implementation to measure the tsc frequency than kernel in some ways.

### Why not use the same mechanism in kernel?

1. I can't, kernel can do many things I can't do in userspace.
2. There are multi ways to calibrate kernel clock, I can't enjoy them directly, what I can do is just catch up the result of kernel clock.

### Why should I calibrate clock by comparing kernel clock?

If I could do better in tsc frequency detection & precision, shouldn't be a good idea that ignore kernel clock?

First, as I mentioned above I need to borrow the abilities of calibration which help to make kernel clock better. 
The crystal of TSC is not as good as we expect, it's just a cheap crystal, we need adjust the frequency result time by time.

Second, it'll make people confused if there are two different clocks and their results are quietly different. 

#### Details of Calibration

1. Using simple linear regression to generate the newest frequency(coefficient) & offset to system clock
2. Using AVX instruction to store coefficient & offset to a specific address
3. Loading coefficient & offset pair when making timestamp

### Drift testings examples

Testing the delta of tsc clock & system clock for each second.

### macOS

platform: macOS Catalina, Intel Core i7-7700HQ

measurement: [tools/longdrift](tools/longdrift/README.md) with default flags.

1. testing time: 100s

<img src="tools/longdrift/longdrift_2021-09-26T011755.PNG" width = "600" height="600"/>

2. testing time: 20mins
 
<img src="tools/longdrift/longdrift_2021-09-26T031816.PNG" width = "600" height="600"/>

3. testing time: 20mins (with Calibrate every 5mins)

<img src="tools/longdrift/longdrift_2021-09-26T034931.PNG" width = "600" height="600"/>

p.s.

1. For macOS, the precision of system clock is just 1us. Which means delta within 1us is almost equal to zero.
2. macOS will update clock in background.

### Linux(1)

platform: Ubuntu 18.04, Intel Core i5-8250U

measurement: [tools/longdrift](tools/longdrift/README.md) with default flags.

1. testing time: 100s

<img src="tools/longdrift/longdrift_2021-09-26T030422.PNG" width = "600" height="600"/>

2. testing time: 20mins

<img src="tools/longdrift/longdrift_2021-09-26T032617.PNG" width = "600" height="600"/>

3. testing time: 20mins (with Calibrate every 5mins)

<img src="tools/longdrift/longdrift_2021-09-26T041218.PNG" width = "600" height="600"/>

4. testing time: 21mins (with Calibrate every 5mins)

<img src="tools/longdrift/longdrift_2021-09-26T044257.PNG" width = "600" height="600"/>

p.s.

It's a cheap laptop, the result is not that good. We could find the crystal frequency wasn't stable enough,
the time sync service really worked hard. For tsc, it's a hard job to catch up the clock too.

## Performance

|OS           |CPU           |benchmark           |    time.Now().UnixNano() ns/op   |  tsc.UnixNano() ns/op    |     delta   |
|--------------------|--------------------|--------------------|----------------|---------------|-------------|
|macOS Catalina |Intel Core i7-7700HQ| BenchmarkUnixNano-8 |    72.8        |  7.65         | -89.49%     |
|Ubuntu 18.04 |Intel Core i5-8250U| BenchmarkUnixNano-8 |    47.7       |  8.41         | -82.36%     |
|Ubuntu 20.04 |Intel Core i9-9920X| BenchmarkUnixNano-8 |    36.5       |  6.19         | -83.04%     |

### TODO new ABI in Go1.17 degrades performance

The new ABI in Go1.17 brings register-based calling, but for assembly codes there is an adapter, it degrades performance around 0.x ns.

Waiting for ABI option to use register-based calling...

## Usage

```go
package main

import (
	"fmt"
	"github.com/templexxx/tsc"
)

func main() {
	ts := tsc.UnixNano()   // Getting unix nano timestamp.
	fmt.Println(ts, tsc.Enabled())  // Print result & tsc enabled or not.
}
```

If `tsc.Enabled() == true`, it'll use tsc register. If not, it'll wrap `time.Now().UnixNano()`.

### Tips

1. Using tools provided by this repo to learn how it works: [calibrate](tools/calibrate/README.md), [longdrift](tools/longdrift/README.md).
And these tools could help you to detect how stable the tsc register & this lib is in your environment.
2. If your application doesn't care the accuracy of clock too much, you could invoke `tsc.ForceTSC()` for allowing unstable frequency.
3. Invoke `tsc.Calibrate()`  periodically if you need to catch up system clock. 5 mins is a good start because the auto NTP adjust is always every 11 mins.
4. Set in-order execution by `tsc.ForbidOutOfOrder()` when you need to measure time cost for short statements.

## Limitation

1. Linux Only: The precision/mechanism of clock on Windows or macOS could not satisfy the tsc frequency detection well enough, it won't be a good idea to use it in production env on Windows/macOS.
2. Intel Enterprise CPU Only: Have tested on Intel platform only, and the testing shows home version's crystal is far away from stable. In other words, the crystal maybe too cheap.
3. Unpredictable behavior on virtual machine: Not sure the TSC sync implementation in certain vm (and cannot access CPUID detection for invariant TSC on some cloud even it has this feature). 

## Reference

1. [Question of linux gettimeofday on StackOverflow](https://stackoverflow.com/questions/13230719/how-is-the-microsecond-time-of-linux-gettimeofday-obtained-and-what-is-its-acc)
2. [Question of TSC frequency variations with temperature on Intel community](https://community.intel.com/t5/Software-Tuning-Performance/TSC-frequency-variations-with-temperature/td-p/1098982)
3. [Question of TSC frequency variations with temperature on Intel community(2)](https://community.intel.com/t5/Software-Tuning-Performance/TSC-frequency-variations-with-temperature/m-p/1126518)