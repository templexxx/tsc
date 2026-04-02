# Calibrate

`calibrate` estimates the TSC (Time Stamp Counter) conversion coefficient:
`coeff = 1 / (frequency / 1e9)`.

This tool is used to validate and tune the coefficient estimation approach used by the `tsc` package.

## Methodology

The current implementation uses simple linear regression with intercept because:

1. Accuracy is sufficient for practical timestamp conversion
2. The model is easily interpretable:
   - The coefficient maps directly to effective TSC frequency
   - The intercept represents the constant offset between TSC and wall clock
