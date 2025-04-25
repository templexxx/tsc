LongDrift
===

longdrift is a tool built for to check TSC clock's drift.

### Drift testing examples

Delta of tsc clock and system clock for each second.

### Linux(1)

platform: Fedora 41, AMD Ryzen 9 7950X3D

1. testing time: 20 mins
   <img src="longdrift_2025-04-26T000854.PNG" width = "600" height="600"/>
2. testing time: 20mins (with Calibrate every 5mins)
   <img src="longdrift_2025-04-26T003635.PNG" width = "600" height="600"/>


### Linux(2)

The result is not that good. We could find the crystal frequency wasn't stable enough;

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
