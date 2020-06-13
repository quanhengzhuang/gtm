package gtm

import (
	"math"
	"time"
)

type Timer interface {
	CalcNextTime(times int, minInterval time.Duration) time.Time
}

type DoubleTimer struct {
}

func (t *DoubleTimer) CalcNextTime(times int, minInterval time.Duration) time.Time {
	interval := time.Duration(math.Pow(2, float64(times)))
	if interval < minInterval {
		interval = minInterval
	}

	return time.Now().Add(interval)
}
