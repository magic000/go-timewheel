package timewheel

import (
	"time"
)

var DefaultTimeWheel *TimeWheel

func init() {
	DefaultTimeWheel, _ = NewTimeWheel(50*time.Millisecond, 100, TickSafeMode())
	DefaultTimeWheel.Start()
}

func ResetDefaultTimeWheel(tw *TimeWheel) {
	tw.Start()
	DefaultTimeWheel = tw
}

func NewTimer(delay time.Duration) *Timer {
	return DefaultTimeWheel.NewTimer(delay)
}

func NewTicker(delay time.Duration) *Ticker {
	return DefaultTimeWheel.NewTicker(delay)
}

func AfterFunc(delay time.Duration, callback func()) *Timer {
	return DefaultTimeWheel.AfterFunc(delay, callback)
}

func After(delay time.Duration) <-chan time.Time {
	return DefaultTimeWheel.After(delay)
}

func Sleep(delay time.Duration) {
	DefaultTimeWheel.Sleep(delay)
}
