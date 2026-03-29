package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubCookieAuth struct {
	validResult bool
	validErr    error
}

func (s stubCookieAuth) Issue() (string, error) {
	return "unused", nil
}

func (s stubCookieAuth) Valid(token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	return s.validResult, s.validErr
}

func TestRequireAuthAllowsRequestWithValidToken(t *testing.T) {
	mw := RequireAuth(stubCookieAuth{validResult: true})
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "token"})
	w := httptest.NewRecorder()

	mw(next).ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestRequireAuthRejectsInvalidToken(t *testing.T) {
	mw := RequireAuth(stubCookieAuth{validResult: false})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected next handler not to be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "token"})
	w := httptest.NewRecorder()

	mw(next).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuthReturnsInternalServerErrorWhenValidationFails(t *testing.T) {
	mw := RequireAuth(stubCookieAuth{validErr: errors.New("boom")})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected next handler not to be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "token"})
	w := httptest.NewRecorder()

	mw(next).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestRequireAuthRejectsMissingCookie(t *testing.T) {
	mw := RequireAuth(stubCookieAuth{validResult: true})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected next handler not to be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	w := httptest.NewRecorder()

	mw(next).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
