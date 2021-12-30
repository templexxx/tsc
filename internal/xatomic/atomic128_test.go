// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xatomic

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"testing"

	"github.com/templexxx/tsc/internal/xbytes"
)

var (
	magic128bits = xbytes.MakeAlignedBlock(16, 64)
)

func init() {
	binary.LittleEndian.PutUint64(magic128bits[:8], 0xdeddeadbeefbeef)
	binary.LittleEndian.PutUint64(magic128bits[8:], 0xdeddeadbeefbeef)
}

func TestAtomicLoad16B(t *testing.T) {
	var x struct {
		before []uint8
		i      []byte
		after  []uint8
	}
	x.before = magic128bits
	x.after = magic128bits
	x.i = xbytes.MakeAlignedBlock(16, 64)

	k := xbytes.MakeAlignedBlock(16, 64)

	for delta := uint64(1); delta+delta > delta; delta += delta {
		AvxLoad16B(&x.i[0], &k[0])
		if !bytes.Equal(k[:], x.i) {
			t.Fatalf("delta=%d i=%d k=%d", delta, x.i, k)
		}

		xi0 := binary.LittleEndian.Uint64(x.i[:8])
		xi0 += delta
		xi1 := binary.LittleEndian.Uint64(x.i[8:])
		xi1 += delta

		binary.LittleEndian.PutUint64(x.i[:8], xi0)
		binary.LittleEndian.PutUint64(x.i[8:], xi1)
	}
	if !bytes.Equal(x.before, magic128bits) || !bytes.Equal(x.after, magic128bits) {
		t.Fatal("wrong magic")
	}
}

func TestAtomicStore16B(t *testing.T) {
	var x struct {
		before []uint8
		i      []byte
		after  []uint8
	}
	x.before = magic128bits
	x.after = magic128bits
	x.i = xbytes.MakeAlignedBlock(16, 64)

	k := xbytes.MakeAlignedBlock(16, 64)
	for delta := uint64(1); delta+delta > delta; delta += delta {
		AvxStore16B(&x.i[0], &k[0])
		if !bytes.Equal(k[:], x.i) {
			t.Fatalf("delta=%d i=%d k=%d", delta, x.i, k)
		}

		xi0 := binary.LittleEndian.Uint64(k[:8])
		xi0 += delta
		xi1 := binary.LittleEndian.Uint64(k[8:])
		xi1 += delta

		binary.LittleEndian.PutUint64(k[:8], xi0)
		binary.LittleEndian.PutUint64(k[8:], xi1)
	}
	if !bytes.Equal(x.before, magic128bits) || !bytes.Equal(x.after, magic128bits) {
		t.Fatal("wrong magic")
	}
}

func TestHammerStoreLoad(t *testing.T) {
	var tests []func(*testing.T, *byte, []byte)
	tests = append(tests, hammerStoreLoadUint128)
	n := int(1e6)
	if testing.Short() {
		n = int(1e4)
	}
	const procs = 8
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(procs))
	for _, tt := range tests {
		c := make(chan int)
		val := xbytes.MakeAlignedBlock(16, 64)
		for p := 0; p < procs; p++ {
			tmp := xbytes.MakeAlignedBlock(16, 64)
			go func() {
				for i := 0; i < n; i++ {
					tt(t, &val[0], tmp)
				}
				c <- 1
			}()
		}
		for p := 0; p < procs; p++ {
			<-c
		}
	}
}

// Tests of correct behavior, with contention.
// (Is the function atomic?)
//
// For each function, we write a "hammer" function that repeatedly
// uses the atomic operation to add 1 to a value. After running
// multiple hammers in parallel, check that we end with the correct
// total.
// Swap can't add 1, so it uses a different scheme.
// The functions repeatedly generate a pseudo-random number such that
// low bits are equal to high bits, swap, check that the old value
// has low and high bits equal.
func hammerStoreLoadUint128(t *testing.T, addr *byte, tmp []byte) {

	AvxLoad16B(addr, &tmp[0])
	v0 := binary.LittleEndian.Uint64(tmp[:8])
	v1 := binary.LittleEndian.Uint64(tmp[8:])

	if v0 != v1 {
		t.Fatalf("Uint128: %#x != %#x", v0, v1)
	}

	binary.LittleEndian.PutUint64(tmp[:8], v0+1)
	binary.LittleEndian.PutUint64(tmp[8:], v1+1)

	AvxStore16B(addr, &tmp[0])
}
