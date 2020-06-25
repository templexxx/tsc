package tsc

import "time"

func GetTS() int64 {
	return getTS()
}

var getTS = func() int64 {
	return time.Now().UnixNano()
}
