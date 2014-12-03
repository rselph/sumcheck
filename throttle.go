package main

import (
	"io"
	"time"
)

type ReadThrottler struct {
	t *Throttler
	r io.Reader
}

func NewReadThrottler(t *Throttler) *ReadThrottler {
	rt := new(ReadThrottler)
	rt.t = t

	return rt
}

func (rt *ReadThrottler) SetReader(r io.Reader) {
	rt.r = r
}

func (rt *ReadThrottler) Start(rate float64) {
	rt.t.Start(rate)
}

func (rt *ReadThrottler) Read(p []byte) (n int, err error) {
	n, err = rt.r.Read(p)
	if rt.t != nil && err == nil {
		rt.t.Tally(int64(n))
		rt.t.Delay()
	}

	return
}

type Throttler struct {
	total     int64
	startTime time.Time
	rate      float64
}

func (t *Throttler) Start(rate float64) {
	t.startTime = time.Now()
	t.total = 0
	t.rate = rate
}

func (t *Throttler) Tally(n int64) {
	t.total += n
}

func (t *Throttler) Delay() {
	if t.rate == 0.0 {
		return
	}

	now := time.Now()

	interval := now.Sub(t.startTime)
	if interval.Seconds() <= 0.0 {
		return
	}
	rateSoFar := float64(t.total) / interval.Seconds()

	if rateSoFar > t.rate {
		shouldHaveTaken := time.Duration(float64(time.Second) * float64(t.total) / t.rate)

		targetTime := t.startTime.Add(shouldHaveTaken)
		time.Sleep(targetTime.Sub(now))
	}
}
