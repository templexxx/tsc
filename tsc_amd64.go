package tsc

import (
	"time"

	"github.com/templexxx/cpu"
)

var start int64

func init() {

	start = time.Now().UnixNano()

	if cpu.X86.HasInvariantTSC {

	}
}

func TscToNano(tsc int64) int64 {
	return start + tsc
}

//go:noescape
func rdtscp() uint64
