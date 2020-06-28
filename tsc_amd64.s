#include "textflag.h"

// func getInOrder() (uint64)
TEXT ·getInOrder(SB), NOSPLIT, $0

	LFENCE             // Ensure all previous instructions have exectuted.
	RDTSC
	LFENCE             // Ensure RDTSC to be exectued prior to exection of any subsequent instruction.
	SALQ $32, DX
	ORQ  DX, AX
	MOVQ AX, ret+0(FP)
	RET

#define tsc AX
#define ftsc X0 // float64(tsc)
#define ns X1 // nanoseconds
#define un R8 // unixNano

// func unixNanoTSC() (int64)
TEXT ·unixNanoTSC(SB), NOSPLIT, $0

	// Both of RSTSC & RDTSCP are not serializing instructions.
	// It does not necessarily wait until all previous instructions
	// have been executed before reading the counter.
	//
	// It's ok to use RSTSC for just getting a timestamp.
	RDTSC        // high 32bit in DX, low 32bit in AX (tsc).
	SALQ $32, DX
	ORQ  DX, tsc // -> [DX, tsc] (high, low)

	VCVTSI2SDQ  tsc, ftsc, ftsc      // ftsc = float64(tsc)
	VMULSD      ·coeff(SB), ftsc, ns // ns = coeff * fstc
	VCVTTSD2SIQ ns, un               // un = int64(ns)
	ADDQ        ·offset(SB), un      // un += offset
	MOVQ        un, ret+0(FP)
	RET
