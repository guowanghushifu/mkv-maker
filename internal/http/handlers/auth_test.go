package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authpkg "github.com/guowanghushifu/mkv-maker/internal/auth"
	"github.com/guowanghushifu/mkv-maker/internal/http/middleware"
)

func TestAuthHandlerLoginSetsSignedCookie(t *testing.T) {
	auth := authpkg.NewCookieAuth("secret", time.Hour)
	handler := &AuthHandler{
		AppPassword:   "secret",
		Auth:          auth,
		SessionMaxAge: 3600,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"secret"}`))
	w := httptest.NewRecorder()
	handler.Login(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Value == "" {
		t.Fatal("expected signed auth cookie")
	}

	valid, err := auth.Valid(cookies[0].Value)
	if err != nil {
		t.Fatalf("Valid returned error: %v", err)
	}
	if !valid {
		t.Fatal("expected issued cookie token to validate")
	}
}

func TestAuthHandlerLogoutClearsCookie(t *testing.T) {
	handler := &AuthHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	w := httptest.NewRecorder()
	handler.Logout(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != middleware.SessionCookieName {
		t.Fatalf("expected cookie name %q, got %q", middleware.SessionCookieName, cookie.Name)
	}
	if cookie.Value != "" {
		t.Fatalf("expected empty cookie value, got %q", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("expected MaxAge -1, got %d", cookie.MaxAge)
	}
	if !cookie.Expires.Equal(time.Unix(0, 0)) {
		t.Fatalf("expected Expires at unix epoch, got %v", cookie.Expires)
	}
}
