package makemkv

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCommandDateOverrideRestoresImmediatelyWhenCommandEndsBeforeWindow(t *testing.T) {
	expireDate := time.Date(2026, 4, 11, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	restoreSignal := make(chan time.Time)
	now := time.Date(2026, 4, 12, 10, 20, 30, 0, expireDate.Location())

	override := NewCommandDateOverride(&expireDate)
	override = override.WithNow(func() time.Time { return now })
	override = override.WithSince(func(time.Time) time.Duration { return 2 * time.Second })
	override = override.WithAfter(func(time.Duration) <-chan time.Time { return restoreSignal })

	var mu sync.Mutex
	var calls []time.Time
	override = override.WithSetSystemDate(func(_ context.Context, target time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, target)
		return nil
	})

	result, err := RunWithCommandDateOverride(override, context.Background(), func(context.Context) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected result ok, got %q", result)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected 2 date changes, got %d", len(calls))
	}
	if got := calls[0].Format("2006-01-02 15:04:05"); got != "2026-03-11 10:20:30" {
		t.Fatalf("expected rollback date to keep clock time, got %s", got)
	}
	if got := calls[1].Format("2006-01-02 15:04:05"); got != "2026-04-12 10:20:32" {
		t.Fatalf("expected immediate restore to use elapsed real time, got %s", got)
	}
}

func TestCommandDateOverrideRestoresAfterWindowEvenWhenCommandKeepsRunning(t *testing.T) {
	expireDate := time.Date(2026, 4, 11, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	restoreSignal := make(chan time.Time, 1)
	release := make(chan struct{})
	restoreObserved := make(chan struct{}, 1)
	now := time.Date(2026, 4, 12, 10, 20, 30, 0, expireDate.Location())

	override := NewCommandDateOverride(&expireDate)
	override = override.WithNow(func() time.Time { return now })
	override = override.WithSince(func(time.Time) time.Duration { return 3 * time.Second })
	override = override.WithAfter(func(time.Duration) <-chan time.Time { return restoreSignal })

	var mu sync.Mutex
	var calls []time.Time
	override = override.WithSetSystemDate(func(_ context.Context, target time.Time) error {
		mu.Lock()
		calls = append(calls, target)
		callCount := len(calls)
		mu.Unlock()
		if callCount == 2 {
			select {
			case restoreObserved <- struct{}{}:
			default:
			}
		}
		return nil
	})

	done := make(chan error, 1)
	go func() {
		_, err := RunWithCommandDateOverride(override, context.Background(), func(context.Context) (string, error) {
			<-release
			return "done", nil
		})
		done <- err
	}()

	restoreSignal <- time.Now()
	<-restoreObserved

	mu.Lock()
	if len(calls) != 2 {
		t.Fatalf("expected restore to happen before command exit, got %d date changes", len(calls))
	}
	if got := calls[1].Format("2006-01-02 15:04:05"); got != "2026-04-12 10:20:33" {
		t.Fatalf("expected timer-based restore after 3 seconds, got %s", got)
	}
	mu.Unlock()

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected restore to run once, got %d date changes", len(calls))
	}
}

func TestCommandDateOverrideIgnoresDateAdjustmentFailure(t *testing.T) {
	expireDate := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	override := NewCommandDateOverride(&expireDate)
	override = override.WithNow(func() time.Time {
		return time.Date(2026, 4, 12, 10, 20, 30, 0, time.UTC)
	})
	override = override.WithAfter(func(time.Duration) <-chan time.Time {
		t.Fatal("did not expect restore timer when rollback failed")
		return nil
	})

	calls := 0
	override = override.WithSetSystemDate(func(_ context.Context, target time.Time) error {
		calls++
		if calls == 1 {
			return errors.New("operation not permitted")
		}
		t.Fatalf("did not expect restore call after rollback failure: %s", target.Format(time.RFC3339))
		return nil
	})

	runCalled := false
	_, err := RunWithCommandDateOverride(override, context.Background(), func(context.Context) (string, error) {
		runCalled = true
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !runCalled {
		t.Fatal("expected wrapped command to run even when date adjustment fails")
	}
	if calls != 1 {
		t.Fatalf("expected exactly one failed rollback attempt, got %d", calls)
	}
}
