#include "textflag.h"

// func AvxLoad16B(src, dst *byte)
TEXT ·AvxLoad16B(SB), NOSPLIT, $0
    MOVQ src+0(FP), AX
    MOVQ dst+8(FP), BX
    VMOVDQA (AX), X3
    VMOVDQA X3, (BX)
	RET

// func AvxStore16B(dst, val *byte)
TEXT ·AvxStore16B(SB),NOSPLIT,$0
	MOVQ dst+0(FP), AX
	MOVQ val+8(FP), BX
	VMOVDQA (BX), X3
	VMOVDQA X3, (AX)
	RET
