package makemkv

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

const commandDateOverrideWindow = 3 * time.Second
const systemDateLayout = "2006-01-02 15:04:05"

type CommandDateOverride struct {
	expireDate     *time.Time
	now            func() time.Time
	since          func(time.Time) time.Duration
	after          func(time.Duration) <-chan time.Time
	setSystemDate  func(context.Context, time.Time) error
	restoreTimeout time.Duration
}

func NewCommandDateOverride(expireDate *time.Time) CommandDateOverride {
	return CommandDateOverride{
		expireDate:     expireDate,
		now:            time.Now,
		since:          time.Since,
		after:          time.After,
		setSystemDate:  setSystemDate,
		restoreTimeout: 5 * time.Second,
	}
}

func (o CommandDateOverride) WithNow(now func() time.Time) CommandDateOverride {
	o.now = now
	return o
}

func (o CommandDateOverride) WithSince(since func(time.Time) time.Duration) CommandDateOverride {
	o.since = since
	return o
}

func (o CommandDateOverride) WithAfter(after func(time.Duration) <-chan time.Time) CommandDateOverride {
	o.after = after
	return o
}

func (o CommandDateOverride) WithSetSystemDate(setter func(context.Context, time.Time) error) CommandDateOverride {
	o.setSystemDate = setter
	return o
}

func (o CommandDateOverride) IsConfigured() bool {
	return o.expireDate != nil || o.now != nil || o.since != nil || o.after != nil || o.setSystemDate != nil || o.restoreTimeout != 0
}

func RunWithCommandDateOverride[T any](o CommandDateOverride, ctx context.Context, run func(context.Context) (T, error)) (T, error) {
	restore := o.begin(ctx)
	result, err := run(ctx)
	if restore != nil {
		restore()
	}
	return result, err
}

func (o CommandDateOverride) begin(ctx context.Context) func() {
	if o.expireDate == nil {
		return nil
	}

	nowFn := o.now
	if nowFn == nil {
		nowFn = time.Now
	}
	current := nowFn()
	if !isDateAfter(current, *o.expireDate) {
		return nil
	}

	setDate := o.setSystemDate
	if setDate == nil {
		setDate = setSystemDate
	}

	rollbackTarget := combineDateAndClock(o.expireDate.AddDate(0, -1, 0), current)
	if err := setDate(ctx, rollbackTarget); err != nil {
		return nil
	}

	sinceFn := o.since
	if sinceFn == nil {
		sinceFn = time.Since
	}
	afterFn := o.after
	if afterFn == nil {
		afterFn = time.After
	}

	done := make(chan struct{})
	var once sync.Once
	restore := func() {
		once.Do(func() {
			close(done)
			realNow := current.Add(sinceFn(current))
			restoreCtx := context.Background()
			if o.restoreTimeout > 0 {
				var cancel context.CancelFunc
				restoreCtx, cancel = context.WithTimeout(context.Background(), o.restoreTimeout)
				defer cancel()
			}
			_ = setDate(restoreCtx, combineDateAndClock(realNow, realNow))
		})
	}

	go func() {
		select {
		case <-afterFn(commandDateOverrideWindow):
			restore()
		case <-done:
		}
	}()

	return restore
}

func isDateAfter(left, right time.Time) bool {
	leftDate := time.Date(left.Year(), left.Month(), left.Day(), 0, 0, 0, 0, left.Location())
	rightDate := time.Date(right.Year(), right.Month(), right.Day(), 0, 0, 0, 0, right.Location())
	return leftDate.After(rightDate)
}

func combineDateAndClock(dateValue, clockValue time.Time) time.Time {
	return time.Date(
		dateValue.Year(),
		dateValue.Month(),
		dateValue.Day(),
		clockValue.Hour(),
		clockValue.Minute(),
		clockValue.Second(),
		0,
		clockValue.Location(),
	)
}

func setSystemDate(ctx context.Context, target time.Time) error {
	return exec.CommandContext(ctx, "date", "-s", target.Format(systemDateLayout)).Run()
}
