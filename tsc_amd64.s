#include "textflag.h"

// func rdtscp() (uint64)
TEXT ·rdtscp(SB), NOSPLIT, $0

    // The RDTSC instruction is not a serializing instruction.
    // It does not necessarily wait until all previous instructions
    // have been executed before reading the counter.
    //
    // RDTSCP doesn't have this issue, but would be slower than RDTSC.
	RDTSCP
	SALQ $32, DX
	ORQ  DX, AX
	MOVQ AX, ret+0(FP)
	RET
