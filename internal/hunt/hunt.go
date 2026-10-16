package hunt

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

var (
	ErrOutOfCapacity = errors.New("out of capacity")
	ErrRateLimited   = errors.New("rate limited")
	ErrTransient     = errors.New("transient failure")
)

type Instance struct {
	ID string
}

type Launcher interface {
	Launch(ctx context.Context) (Instance, error)
}

type Attempt struct {
	Number int
	Err    error
	Wait   time.Duration
}

type Hunter struct {
	Launcher   Launcher
	Interval   time.Duration
	MaxBackoff time.Duration
	OnAttempt  func(Attempt)
}

func (h *Hunter) Run(ctx context.Context) (Instance, error) {
	backoff := h.Interval

	for number := 1; ; number++ {
		instance, err := h.Launcher.Launch(ctx)
		if err == nil {
			h.report(Attempt{Number: number})
			return instance, nil
		}

		switch {
		case errors.Is(err, ErrRateLimited):
			backoff = min(backoff*2, h.MaxBackoff)
		case errors.Is(err, ErrOutOfCapacity), errors.Is(err, ErrTransient):
			backoff = h.Interval
		default:
			h.report(Attempt{Number: number, Err: err})
			return Instance{}, err
		}

		wait := withJitter(backoff)
		h.report(Attempt{Number: number, Err: err, Wait: wait})

		if err := sleep(ctx, wait); err != nil {
			return Instance{}, err
		}
	}
}

func (h *Hunter) report(a Attempt) {
	if h.OnAttempt != nil {
		h.OnAttempt(a)
	}
}

func withJitter(d time.Duration) time.Duration {
	return d + time.Duration(rand.Int64N(int64(d)/10+1))
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
