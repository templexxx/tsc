package tsc

var (
	supported int64 = 0	// Supported invariant TSC or not.
	stable int64 = 0 	// TSC frequency is stable or not. (If not, we may have big gap between wall clock after long run)
	forceTSC int64 = 0	// Enable TSC no matter it's stable or not.
	enabled int64 = 0	// TSC clock source is enabled or not, if yes, getting timestamp by tsc register.
)

// FreqSource is the source of tsc frequency.
// Empty means no available source or could not passing fast clock delta checking.
var FreqSource = ""

// FreqEnv is the TSC frequency calculated by tools/getfreq or other tool.
// It'll help
const FreqEnv = "TSC_FREQ_X"

const (
	// EnvSource means this lib gets tsc frequency from environment variable.
	EnvSource = "env"
	// CPUFeatureSource means this lib gets tsc frequency from https://github.com/templexxx/cpu
	//
	// The frequency provided by Intel manual is not that reliable,
	// (Actually, there is 1/millions delta at least)
	// it's easy to ensure that, because it's common that crystal won't work in the frequency we expected
	// Yes, we can get an expensive crystal, but we can't replace the crystal in CPU by the better one.
	// That's why we have to adjust the frequency by tools provided by this project/other ways.
	CPUFeatureSource = "cpu_feat"
	// FastDetectSource means this lib get tsc frequency from a fast detection.
	FastDetectSource = "fast_detect"
)

// UnixNano returns t as a Unix time, the number of nanoseconds elapsed
// since January 1, 1970 UTC.
//
// Warn:
// UnixNano is not designed for benchmarking purpose,
// DO NOT use it for measuring function performance.
//
// e.g.
// ```
// 	start := tsc.UnixNano()
// 	foo()
// 	end := tsc.UnixNano()
// 	cost := end - start
// ```
// The value of cost is unpredictable,
// because all instructions for getting tsc are not serializing,
// we need to be careful to deal with the order (use barrier).
//
// Although I have implemented a tsc getting method with order,
// Go is not designed for HPC, and it's own benchmark testing is
// enough in most cases. I think I should go further to explore to
// make sure it's best practice for counting cycles.
// See GetInOrder in tsc_amd64.s for more details.
func UnixNano() int64 {
	return unixNano()
}
// Enabled indicates TSC clock source is enabled or not (using TSC register as clock source).
// If true, use TSC time. Otherwise, use time.Now().
func Enabled() bool {
	return enabled == 1
}

// Supported indicates Invariant TSC supported.
func Supported() bool {
	return supported == 1
}

// Stable indicates TSC frequency is stable or not. (If not, we may have big gap between wall clock after long run)
func Stable() bool {
	return stable == 1
}

// ForceTSC forces using TSC as clock source.
// Returns false if TSC is unsupported.
//
// Warn:
// Not thread safe, using it at application starting.
func ForceTSC() bool {

	if Enabled() {
		return true
	}

	if !Supported() {
		return false
	}

	forceTSC = 1

	return reset()
}

func isEven(n int) bool {
	return n&1 == 0
}
