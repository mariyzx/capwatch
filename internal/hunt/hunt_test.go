package hunt

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

const (
	interval   = time.Millisecond
	maxBackoff = 4 * time.Millisecond
)

type fakeLauncher struct {
	errs  []error
	calls int
}

func (f *fakeLauncher) Launch(ctx context.Context) (Instance, error) {
	f.calls++
	if len(f.errs) == 0 {
		return Instance{ID: "ocid1.instance.test"}, nil
	}
	err := f.errs[0]
	f.errs = f.errs[1:]
	return Instance{}, err
}

func newHunter(l Launcher, attempts *[]Attempt) *Hunter {
	return &Hunter{
		Launcher:   l,
		Interval:   interval,
		MaxBackoff: maxBackoff,
		OnAttempt:  func(a Attempt) { *attempts = append(*attempts, a) },
	}
}

func launchErr(err error) error {
	return fmt.Errorf("launching instance: %w", err)
}

func TestRunRetriesUntilLaunchSucceeds(t *testing.T) {
	tests := []struct {
		name string
		errs []error
	}{
		{"no errors", nil},
		{"out of capacity", []error{launchErr(ErrOutOfCapacity), launchErr(ErrOutOfCapacity)}},
		{"transient", []error{launchErr(ErrTransient)}},
		{"rate limited", []error{launchErr(ErrRateLimited)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			launcher := &fakeLauncher{errs: tt.errs}
			var attempts []Attempt

			instance, err := newHunter(launcher, &attempts).Run(context.Background())

			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if instance.ID != "ocid1.instance.test" {
				t.Errorf("instance.ID = %q", instance.ID)
			}
			if want := len(tt.errs) + 1; launcher.calls != want {
				t.Errorf("launcher calls = %d, want %d", launcher.calls, want)
			}
			if last := attempts[len(attempts)-1]; last.Err != nil || last.Wait != 0 {
				t.Errorf("last attempt = %+v, want success without wait", last)
			}
		})
	}
}

func TestRunStopsOnUnexpectedError(t *testing.T) {
	unexpected := errors.New("not authorized")
	launcher := &fakeLauncher{errs: []error{launchErr(ErrOutOfCapacity), unexpected}}
	var attempts []Attempt

	_, err := newHunter(launcher, &attempts).Run(context.Background())

	if !errors.Is(err, unexpected) {
		t.Fatalf("Run() error = %v, want %v", err, unexpected)
	}
	if launcher.calls != 2 {
		t.Errorf("launcher calls = %d, want 2", launcher.calls)
	}
}

func TestRunBacksOffOnRateLimit(t *testing.T) {
	launcher := &fakeLauncher{errs: []error{
		launchErr(ErrRateLimited),
		launchErr(ErrRateLimited),
		launchErr(ErrRateLimited),
		launchErr(ErrOutOfCapacity),
	}}
	var attempts []Attempt

	if _, err := newHunter(launcher, &attempts).Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantBase := []time.Duration{2 * interval, maxBackoff, maxBackoff, interval}
	for i, base := range wantBase {
		if wait := attempts[i].Wait; wait < base || wait > base+base/10 {
			t.Errorf("attempt %d wait = %v, want between %v and %v", i+1, wait, base, base+base/10)
		}
	}
}

func TestRunStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	launcher := &fakeLauncher{errs: []error{launchErr(ErrOutOfCapacity)}}
	hunter := &Hunter{
		Launcher:   launcher,
		Interval:   time.Hour,
		MaxBackoff: time.Hour,
		OnAttempt:  func(Attempt) { cancel() },
	}

	_, err := hunter.Run(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want %v", err, context.Canceled)
	}
}
