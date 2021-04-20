package tsc

// FreqTbl is the tsc frequency tables.
// You can record the tsc frequency tables.
//
// key: <cpu.X86.Signature>_<cpu.X86.SteppingID>
// value: frequency (Hz)
//
// <cpu.X86.Signature>_<cpu.X86.SteppingID> is enough to get a tsc frequency,
// it means CPU family, model and its micro architecture.
var FreqTbl = map[string]float64{
	"06_9EH_2": 2999998888.73,
	"06_55H_5": 3700008733,
}
