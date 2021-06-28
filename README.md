# tsc
Get unix time (nanoseconds) in blazing low latency. About 10x~100x faster than time.Now().UnixNano(). 

- __Time Stamp Counter (TSC)__

  Based on CPU's TSC register. With Invariant TSC supports, we could get a reliable frequency even cross multi cores/CPUs.

- __Low Latency__

  Under 10ns to get each timestamp.
  
- __Stable__

  Unlike time.Now(), the latency of tsc is stable.
  
  Although time.Now() is using VDSO to get time, but it's unstable,
  sometimes it will take more than 1000ns.

- __High Precision__

  tsc's precision is nanosecond.

- __Calibrate__

  Could be calibrated according to wall clock periodically, no worry about NTP. More details: `func Calibrate()` in [tsc_amd64.go](tsc_amd64.go)

## Performance

|OS           |CPU           |benchmark           |    time.Now().UnixNano() ns/op   |  tsc.UnixNano() ns/op    |     delta   |
|--------------------|--------------------|--------------------|----------------|---------------|-------------|
|MacOS Catalina |Intel Core i7-7700HQ| BenchmarkUnixNano-8 |    72.8        |  7.65         | -89.49%     |
|Ubuntu 18.04 |Intel Core i5-8250U| BenchmarkUnixNano-8 |    47.7       |  8.41         | -82.36%     |
|Ubuntu 20.04 |Intel Core i9-9920X| BenchmarkUnixNano-8 |    36.5       |  6.19         | -83.04%     |

## Preparation

If you need a really accurate clock, you should run [getfreq](tools/getfreq) first to get TSC frequency, then use [longdrift](tools/longdrift)
to testing the frequency. You could find the best interval for invoking `tsc.Calibrate()` by [longdrift](tools/longdrift) too.

After testing, you should set env_var(`TSC_FREQ_X`) to the best frequency for each server.

If your application doesn't care the accuracy of clock too much, you could invoke `tsc.ResetEnabled(true)` for allowing unstable frequency.
Although it's "unstable", we still run a simple checking for ensuring the result won't be too bad. 

Get a more accurate TSC frequency is important for getting a closer result, which means within a duration, the drift between standard clock
and TSC clock won't be too bad. Actually, even with an "accurate" TSC frequency, we could still get a bad result after a long run. 
That caused by the unstable crystal frequency (both of wall clock crystal & tsc crystal), the frequency is a wave, but what we got was
just a point of the wave. `tsc.Calibrate()`  could help to get an offset to fix the drift which getting bigger and bigger. The drift must
be bigger and bigger, it's easy to understand it by the formula: `tsc_clock = tsc_value * coefficient(point_frequency) + offset`, when the
coefficient fixed, as time goes by, the drift must get bigger.

## Usage

```go
package main

import "github.com/templexxx/tsc"

func main() {
	ts := tsc.UnixNano()   
	...
}
```

If `tsc.Enabled() == true`, it'll use tsc register. If not, it'll wrap `time.Now().UnixNano()`.

## Warning

The crystal used by TSC is not that stable(and testing result maybe not reliable, because of
the SMI), what's worse the frequency given by Intel manual maybe far away from the real frequency 
(even same CPU model may have different frequency, and the delta is big enough to make your clock too slower/faster).

Before using this lib, you should be sure what you need. If you really want a clock with high performance, please testing the tsc clock carefully.
(there is a tool [longdrift](tools/longdrift) could help you observe the frequency wave. It's a good practice that run this tool on each server with
different options at least one day.)

## Limitation

>- Linux Only
> 
>   The precision of clock on Windows or macOS could not satisfy the tsc frequency detection well enough.
> 
>- Intel Only
>
>   Only tested on Intel platform.
>
>- Invariant TSC supports
>   
>   Invariant TSC could make sure TSC got synced among multi CPUs.
>   They will be reset at same time, and run in same frequency.
>
>- Reliable TSC frequency 
> 
>   The TSC frequency provided by Intel CPU must be accurate. 
>
>- AVX supports
>
>   Some instructions need AVX, see [tsc_amd64.s](tsc_amd64.s) for details.
>
>- No support on virtual machine
>
>   Have tested on AWS EC2, because of CPUID.15H is disabled. And tsc may won't work as we expect on a virtual machine.
>
>- Handle Limitation
>
>   If tsc can't be enabled, it will use time.Now().UnixNano() automatically.