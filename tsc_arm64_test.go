//go:build arm64

package tsc

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/templexxx/tsc/internal/xbytes"
)

func TestStoreOffsetCoeff(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	dst := xbytes.MakeAlignedBlock(128, 128)
	for i := 0; i < 1024; i++ {
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
	if !Supported() {
		t.Skip("Generic Timer is unsupported")
	}

	n := 4096
	ret0 := make([]int64, n)
	ret1 := make([]int64, n)

	for i := range ret0 {
		ret0[i] = GetInOrder()
		ret1[i] = GetInOrder()
	}

	cnt := 0
	for i := 0; i < n; i++ {
		d := ret1[i] - ret0[i]
		if d < 0 {
			cnt++
		}
	}
	if cnt > 0 {
		t.Fatal(fmt.Sprintf("GetInOrder is not in order: %d aren't in order", cnt))
	}
}

func TestReadCounterFrequency(t *testing.T) {
	if !Supported() {
		t.Skip("Generic Timer is unsupported")
	}

	freq := readCounterFrequency()
	if freq == 0 {
		t.Fatal("counter frequency is 0")
	}
	t.Logf("ARM64 Generic Timer frequency: %d Hz", freq)
}

func BenchmarkGetInOrder(b *testing.B) {
	if !Supported() {
		b.Skip("Generic Timer is unsupported")
	}

	for i := 0; i < b.N; i++ {
		_ = GetInOrder()
	}
}

func BenchmarkRDTSC(b *testing.B) {
	if !Supported() {
		b.Skip("Generic Timer is unsupported")
	}

	for i := 0; i < b.N; i++ {
		_ = RDTSC()
	}
}

func BenchmarkUnixNanoARMFMADD(b *testing.B) {
	if !Supported() {
		b.Skip("Generic Timer is unsupported")
	}

	for i := 0; i < b.N; i++ {
		_ = unixNanoARMFMADD()
	}
}

func BenchmarkUnixNanoARM16B(b *testing.B) {
	if !Supported() {
		b.Skip("Generic Timer is unsupported")
	}

	for i := 0; i < b.N; i++ {
		_ = unixNanoARM16B()
	}
}

func BenchmarkUnixNanoARM16Bfence(b *testing.B) {
	if !Supported() {
		b.Skip("Generic Timer is unsupported")
	}

	for i := 0; i < b.N; i++ {
		_ = unixNanoARM16Bfence()
	}
}
