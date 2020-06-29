# tsc
Get unix time (nanoseconds) in blazing low latency. About 10x~100x faster than time.Now().UnixNano().

- __Time Stamp Counter (TSC)__

  Based on CPU's TSC register.

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

## Clock Offset

The offset between wall clock and tsc is extremely low (under dozens ns in avg, maximum is hundreds-1000 ns), see [test codes](tsc_test.go) for more details.


## Limitation

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