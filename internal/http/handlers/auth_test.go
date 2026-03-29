package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authpkg "github.com/guowanghushifu/mkv-maker/internal/auth"
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
}
