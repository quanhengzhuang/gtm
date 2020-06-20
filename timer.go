package gtm

import (
	"math"
	"time"
)

type Timer interface {
	CalcRetryTime(times int, minInterval time.Duration) time.Time
}

type DoubleTimer struct {
}

func (t *DoubleTimer) CalcRetryTime(times int, minInterval time.Duration) time.Time {
	interval := time.Duration(math.Pow(2, float64(times))) * time.Second
	if interval < minInterval {
		interval = minInterval
	}

	return time.Now().Add(interval)
}
