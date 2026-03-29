package auth

import (
	"testing"
	"time"
)

func TestCookieAuthIssueAndValidate(t *testing.T) {
	auth := NewCookieAuth("app-password", time.Hour)

	token, err := auth.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	valid, err := auth.Valid(token)
	if err != nil {
		t.Fatalf("Valid returned error: %v", err)
	}
	if !valid {
		t.Fatal("expected issued token to validate")
	}
}

func TestCookieAuthRejectsTamperedToken(t *testing.T) {
	auth := NewCookieAuth("app-password", time.Hour)

	token, err := auth.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	valid, err := auth.Valid(token + "x")
	if err != nil {
		t.Fatalf("Valid returned error: %v", err)
	}
	if valid {
		t.Fatal("expected tampered token to be rejected")
	}
}

func TestCookieAuthRejectsExpiredToken(t *testing.T) {
	auth := NewCookieAuth("app-password", 0)

	token, err := auth.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	valid, err := auth.Valid(token)
	if err != nil {
		t.Fatalf("Valid returned error: %v", err)
	}
	if valid {
		t.Fatal("expected token to be expired")
	}
}

func TestCookieAuthExpiresAtBoundary(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	auth := NewCookieAuth("app-password", time.Second)
	auth.clockNow = func() time.Time { return base }

	token, err := auth.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	auth.clockNow = func() time.Time { return base.Add(999 * time.Millisecond) }
	valid, err := auth.Valid(token)
	if err != nil {
		t.Fatalf("Valid returned error before expiry: %v", err)
	}
	if !valid {
		t.Fatal("expected token to be valid before expiry boundary")
	}

	auth.clockNow = func() time.Time { return base.Add(time.Second) }
	valid, err = auth.Valid(token)
	if err != nil {
		t.Fatalf("Valid returned error at expiry boundary: %v", err)
	}
	if valid {
		t.Fatal("expected token to be invalid at expiry boundary")
	}
}
