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

  time.Now()'s precision is only µs.

  tsc is ns.

- __Calibrate__

  Could be calibrated according to wall clock periodically, no worry about NTP. More details: `func Calibrate()` in [tsc_amd64.go](tsc_amd64.go)

## Performance

**Platform:** 

*MacBook Pro (15-inch, 2017) 2.8 GHz Quad-Core Intel Core i7-7700HQ*

|benchmark           |    time.Now() ns/op   |  tsc ns/op    |     delta   |
|--------------------|----------------|---------------|-------------|
|BenchmarkUnixNano-8 |    72.8        |  7.65         | -89.49%     |


## Clock Offset

The offset between wall clock and tsc is extremely low (under dozens ns in avg, maximum is hundreds ns), see [test codes](tsc_test.go) for more details.


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