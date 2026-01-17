# Calibrate

Calibrate is a tool for determining the TSC (Time Stamp Counter) coefficient, calculated as `1 / (frequency / 1e9)`.

This utility helps finalize the algorithm for TSC coefficient calculation in the tsc package.

## Methodology

Simple linear regression with intercept was chosen for the following reasons:

1. The results provide sufficient accuracy for practical applications
2. The model is easily interpretable:
   - The coefficient directly corresponds to the frequency
   - The intercept represents the constant offset between the two clock sources
