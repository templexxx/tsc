//go:build amd64

package tsc

import (
	"math/rand"
	"testing"

	"github.com/templexxx/tsc/internal/xbytes"
)

func TestStoreOffsetCoeff(t *testing.T) {
	t.Parallel()

	dst := xbytes.MakeAlignedBlock(128, 128)

	for range 1024 {
		coeff := rand.Float64()
		offset := rand.Int63()
		storeOffsetCoeff(&dst[0], offset, coeff)

		actOffset, actCoeff := LoadOffsetCoeff(&dst[0])
		if actOffset != offset {
			t.Log(coeff, offset, actCoeff, actOffset)
			t.Fatalf("offset not equal, exp: %d, got: %d", offset, actOffset)
		}

		if actCoeff != coeff {
			t.Fatalf("coeff not equal, exp: %.2f, got: %.2f", coeff, actCoeff)
		}
	}
}

// Out-of-Order test, GetInOrder should be in order as we assume.
func TestGetInOrder(t *testing.T) {
	t.Parallel()

	if !Supported() {
		t.Skip("tsc is unsupported")
	}

	n := 4096
	ret0 := make([]int64, n)
	ret1 := make([]int64, n)

	for i := range ret0 {
		ret0[i] = GetInOrder()
		ret1[i] = GetInOrder()
	}

	cnt := 0

	for i := range n {
		d := ret1[i] - ret0[i]
		if d < 0 {
			cnt++
		}
	}

	if cnt > 0 {
		t.Fatalf("GetInOrder is not in order: %d aren't in order", cnt)
	}
}

func BenchmarkGetInOrder(b *testing.B) {
	if !Supported() {
		b.Skip("tsc is unsupported")
	}

	for range b.N {
		_ = GetInOrder()
	}
}

func BenchmarkRDTSC(b *testing.B) {
	if !Supported() {
		b.Skip("tsc is unsupported")
	}

	for range b.N {
		_ = RDTSC()
	}
}

func BenchmarkUnixNanoTSCFMA(b *testing.B) {
	if !Supported() {
		b.Skip("tsc is unsupported")
	}

	for range b.N {
		_ = unixNanoTSCFMA()
	}
}

func BenchmarkUnixNanoTSC16B(b *testing.B) {
	if !Supported() {
		b.Skip("tsc is unsupported")
	}

	for range b.N {
		_ = unixNanoTSC16B()
	}
}
