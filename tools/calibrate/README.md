Calibrate
===

Calibrate is a tool for showing attempts to find out TSC coefficient (`1 / (frequency / 1e9)`).

It helps me to finalize the algorithm of calculation of TSC coefficient in tsc package.

Reasons of choosing simple linear regression with intercept:

1. Easy to calculate.
2. The result is good enough.
3. Easy to understand, the coefficient represents the `freqeuncy`, the `offset` is two clocks constant offset.