LongDrift
===

`longdrift` is a tool for measuring long-running drift between TSC-based time and system time.

The accuracy and stability of TSC-based time depend mainly on CPU clock stability and calibration interval.

### Drift testing examples

Each chart shows the per-second delta between TSC clock and system clock.

### Linux(1)

platform: Fedora 41, AMD Ryzen 9 7950X3D
1. testing time: 82,800 seconds (with Calibrate every 5 mins):
```shell
./longdrift --enable_calibrate=true --job_time=82800
job start at: 2025-04-26T01:46:37.100077215+08:00
testing with options: -calibrate_interval 300 -cmp_sys false -coeff 0 -enable_calibrate true -idle true -in_order false -job_time 82800 -print false -threads 1
cpu: 19_61H_2, begin with tsc_freq: 4200049623.1846308708190918(coeff: 0.2380924250227700), offset: 1745054585295363584
[thread-0] delta(abs): first: 1.10us, last: 1.05us, min: 0.00us, max: 10.75us, mean: 1.18us
job taken: 23h0m29.119050804s
```
   <img src="longdrift_2025-04-27T004706.PNG" width = "600" height="600"/>
2. testing time: 20 mins
   <img src="longdrift_2025-04-26T000854.PNG" width = "600" height="600"/>
3. testing time: 20 mins (with Calibrate every 5 mins)
   <img src="longdrift_2025-04-26T003635.PNG" width = "600" height="600"/>


### Linux(2)

This result is less stable, indicating the crystal frequency is not sufficiently stable for this setup.

platform: Ubuntu 18.04, Intel Core i5-8250U

1. testing time: 100s

<img src="longdrift_2021-09-26T030422.PNG" width = "600" height="600"/>

2. testing time: 20mins

<img src="longdrift_2021-09-26T032617.PNG" width = "600" height="600"/>

3. testing time: 20mins (with Calibrate every 5mins)

<img src="longdrift_2021-09-26T041218.PNG" width = "600" height="600"/>

4. testing time: 21mins (with Calibrate every 5mins)

<img src="longdrift_2021-09-26T044257.PNG" width = "600" height="600"/>

### macOS

platform: macOS Catalina, Intel Core i7-7700HQ

1. testing time: 100s

<img src="longdrift_2021-09-26T011755.PNG" width = "600" height="600"/>

2. testing time: 20mins

<img src="longdrift_2021-09-26T031816.PNG" width = "600" height="600"/>

3. testing time: 20mins (with Calibrate every 5mins)

<img src="longdrift_2021-09-26T034931.PNG" width = "600" height="600"/>
