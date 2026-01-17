//go:build arm64

#include "textflag.h"

// func GetInOrder() int64
TEXT ·GetInOrder(SB), NOSPLIT, $0-8
	// ISB ensures all previous instructions have executed
	WORD $0xD5033FDF  // ISB

	// Read the virtual counter (CNTVCT_EL0)
	WORD $0xD53BE040  // MRS CNTVCT_EL0, R0

	// ISB ensures counter read completes before subsequent instructions
	WORD $0xD5033FDF  // ISB

	MOVD R0, ret+0(FP)
	RET

// func RDTSC() int64
TEXT ·RDTSC(SB), NOSPLIT, $0-8
	// Read the virtual counter without barriers (fast path)
	WORD $0xD53BE040  // MRS CNTVCT_EL0, R0

	MOVD R0, ret+0(FP)
	RET

// func readCounterFrequency() int64
TEXT ·readCounterFrequency(SB), NOSPLIT, $0-8
	// Read the counter frequency register (CNTFRQ_EL0)
	WORD $0xD53BE000  // MRS CNTFRQ_EL0, R0

	MOVD R0, ret+0(FP)
	RET

// func unixNanoARM16B() int64
TEXT ·unixNanoARM16B(SB), NOSPLIT, $0-8
	// Read counter without barriers (fast path)
	WORD $0xD53BE040  // MRS CNTVCT_EL0, R0

	// Load offset and coefficient from memory
	MOVD ·OffsetCoeffAddr(SB), R1

	// Load coeff from [R1] and offset from [R1+8]
	FMOVD (R1), F0
	MOVD 8(R1), R2

	// Convert counter to float64 (unsigned)
	WORD $0x9E630001  // UCVTF D1, X0 (unsigned conversion)

	// Multiply: ns = coeff * counter
	WORD $0x1E600820  // FMULD D0, D1, D0

	// Convert to int64
	WORD $0x9E780000  // FCVTZS D0, X0

	// Add offset: result = ns + offset
	ADD R2, R0

	MOVD R0, ret+0(FP)
	RET

// func unixNanoARMFMADD() int64
TEXT ·unixNanoARMFMADD(SB), NOSPLIT, $0-8
	// Read counter without barriers
	WORD $0xD53BE040  // MRS CNTVCT_EL0, R0

	// Load offset and coefficient
	MOVD ·OffsetCoeffFAddr(SB), R1

	// Load coeff from [R1] and offset from [R1+8]
	FMOVD (R1), F0    // coeff
	FMOVD 8(R1), F2   // offset

	// Convert counter to float64 (unsigned)
	WORD $0x9E630001  // UCVTF D1, X0 (unsigned conversion)

	// FMADD: D2 = D2 + D0 * D1 (offset + coeff * counter)
	WORD $0x1F000C02  // FMADD D2, D0, D1, D2

	// Convert to int64
	WORD $0x9E780040  // FCVTZS D2, X0

	MOVD R0, ret+0(FP)
	RET

// func unixNanoARM16Bfence() int64
TEXT ·unixNanoARM16Bfence(SB), NOSPLIT, $0-8
	// ISB before reading counter
	WORD $0xD5033FDF  // ISB

	// Read counter
	WORD $0xD53BE040  // MRS CNTVCT_EL0, R0

	// ISB after reading counter
	WORD $0xD5033FDF  // ISB

	// Load offset and coefficient
	MOVD ·OffsetCoeffAddr(SB), R1

	// Load coeff from [R1] and offset from [R1+8]
	FMOVD (R1), F0
	MOVD 8(R1), R2

	// Convert counter to float64 (unsigned)
	WORD $0x9E630001  // UCVTF D1, X0 (unsigned conversion)

	// Multiply: ns = coeff * counter
	WORD $0x1E600820  // FMULD D0, D1, D0

	// Convert to int64
	WORD $0x9E780000  // FCVTZS D0, X0

	// Add offset
	ADD R2, R0

	MOVD R0, ret+0(FP)
	RET

// func storeOffsetCoeff(dst *byte, offset int64, coeff float64)
TEXT ·storeOffsetCoeff(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0
	MOVD offset+8(FP), R1
	FMOVD coeff+16(FP), F0

	// Store coeff at [R0] and offset at [R0+8]
	FMOVD F0, (R0)
	MOVD R1, 8(R0)

	RET

// func storeOffsetFCoeff(dst *byte, offset, coeff float64)
TEXT ·storeOffsetFCoeff(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0
	FMOVD offset+8(FP), F0
	FMOVD coeff+16(FP), F1

	// Store coeff at [R0] and offset at [R0+8]
	FMOVD F1, (R0)
	FMOVD F0, 8(R0)

	RET

// func LoadOffsetCoeff(src *byte) (offset int64, coeff float64)
TEXT ·LoadOffsetCoeff(SB), NOSPLIT, $0-24
	MOVD src+0(FP), R0

	// Load coeff from [R0] and offset from [R0+8]
	FMOVD (R0), F0
	MOVD 8(R0), R1

	// Return values
	MOVD R1, offset+8(FP)
	FMOVD F0, coeff+16(FP)

	RET
