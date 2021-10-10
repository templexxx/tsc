#include "textflag.h"

// func GetInOrder() int64
TEXT ·GetInOrder(SB), NOSPLIT, $0

	LFENCE             // Ensure all previous instructions have exectuted.
	RDTSC
	LFENCE             // Ensure RDTSC to be exectued prior to exection of any subsequent instruction.
	SALQ $32, DX
	ORQ  DX, AX
	MOVQ AX, ret+0(FP)
	RET

// func RDTSC() int64
TEXT ·RDTSC(SB), NOSPLIT, $0

	RDTSC
	SALQ $32, DX
	ORQ  DX, AX
	MOVQ AX, ret+0(FP)
	RET

// func unixNanoTSC16B() int64
TEXT ·unixNanoTSC16B(SB), NOSPLIT, $0

	// Both of RSTSC & RDTSCP are not serializing instructions.
	// It does not necessarily wait until all previous instructions
	// have been executed before reading the counter.
	//
	// It's ok to use RSTSC for just getting a timestamp.
	RDTSC        // high 32bit in DX, low 32bit in AX (tsc).
	SALQ $32, DX
	ORQ  DX, AX  // -> [DX, tsc] (high, low)

	VCVTSI2SDQ  AX, X0, X0               // ftsc = float64(tsc)
	MOVQ        ·OffsetCoeffAddr(SB), BX
	VMOVDQA     (BX), X3
	VMULSD      X3, X0, X0               // ns = coeff * ftsc
	VCVTTSD2SIQ X0, AX                   // un = int64(ns)
	VMOVHLPS    X3, X3, X3
	VMOVQ       X3, CX
	ADDQ        CX, AX                   // un += offset
	MOVQ        AX, ret+0(FP)
	RET

// func unixNanoTSC16Bfence() int64
TEXT ·unixNanoTSC16Bfence(SB), NOSPLIT, $0

	LFENCE
	RDTSC        // high 32bit in DX, low 32bit in AX (tsc).
	LFENCE
	SALQ $32, DX
	ORQ  DX, AX  // -> [DX, tsc] (high, low)

	VCVTSI2SDQ  AX, X0, X0               // ftsc = float64(tsc)
	MOVQ        ·OffsetCoeffAddr(SB), BX
	VMOVDQA     (BX), X3
	VMULSD      X3, X0, X0               // ns = coeff * ftsc
	VCVTTSD2SIQ X0, AX                   // un = int64(ns)
	VMOVHLPS    X3, X3, X3
	VMOVQ       X3, CX
	ADDQ        CX, AX                   // un += offset
	MOVQ        AX, ret+0(FP)
	RET

// func loadOffsetCoeff(src *byte) (offset int64, coeff float64)
TEXT ·LoadOffsetCoeff(SB), NOSPLIT, $0
	MOVQ     src+0(FP), AX
	VMOVDQA  (AX), X0
	VMOVQ    X0, BX
	VMOVHLPS X0, X0, X0
	VMOVQ    X0, CX
	MOVQ     CX, offset+8(FP)
	MOVQ     BX, coeff+16(FP)
	RET

// func storeOffsetCoeff(dst *byte, offset int64, coeff float64)
TEXT ·storeOffsetCoeff(SB), NOSPLIT, $0
	MOVQ    dst+0(FP), AX
	VMOVQ   coeff+16(FP), X5
	VMOVHPS offset+8(FP), X5, X4
	VMOVDQA X4, (AX)
	RET
