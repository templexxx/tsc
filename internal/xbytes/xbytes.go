// Copyright (C) 2012 by Nick Craig-Wood http://www.craig-wood.com/nick/
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package xbytes provides abilities to make Aligned bytes slice.

package xbytes

import "unsafe"

// Alignment returns Alignment of the block in memory
// with reference to alignSize.
//
// Can't check Alignment of a zero sized block as &block[0] is invalid.
func Alignment(block []byte, alignSize int) int {
	return int(uintptr(unsafe.Pointer(&block[0])) & uintptr(alignSize-1))
}

// MakeAlignedBlock returns []byte of size BlockSize aligned to a multiple
// of alignSize in memory (must be power of two).
func MakeAlignedBlock(blockSize, alignSize int) []byte {
	block := make([]byte, blockSize+alignSize)
	if alignSize == 0 {
		return block
	}

	a := Alignment(block, alignSize)

	offset := 0
	if a != 0 {
		offset = alignSize - a
	}

	block = block[offset : offset+blockSize]
	// Can't check Alignment of a zero sized block.
	if blockSize != 0 {
		a = Alignment(block, alignSize)
		if a != 0 {
			panic("failed to align block")
		}
	}

	return block
}
