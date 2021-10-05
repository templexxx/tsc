package xatomic

// AvxLoad16B atomically loads 16bytes from *addr.
// src & dst must be cache line aligned.
//
//go:noescape
func AvxLoad16B(src, dst *byte)

// AvxStore16B atomically stores 16bytes to *addr.
// src & val must be cache line aligned.
//
//go:noescape
func AvxStore16B(src, val *byte)
