Calibrate
===

Calibrate is a tool for showing attempts to find out TSC coefficient (`1 / (frequency / 1e9)`).

It helps me to finalize the algorithm of calculation of TSC coefficient in tsc package.

Reasons of choosing simple linear regression without intercept:

1. Easy to calculate.
2. Only coefficient will be updated in the regular calibration, easy to implement atomic operation on it for future calibrate.
3. The result is good enough.