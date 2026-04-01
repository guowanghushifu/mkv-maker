package handlers

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/guowanghushifu/mkv-maker/internal/http/middleware"
)

type TokenIssuer interface {
	Issue() (string, error)
}

type AuthHandler struct {
	AppPassword   string
	Auth          TokenIssuer
	SessionMaxAge int
	SessionSecure bool
}

type loginRequest struct {
	Password string `json:"password"`
}

const loginBodyLimit = 4 << 10

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if !decodeJSONBodyLimited(w, r, loginBodyLimit, &req) {
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.AppPassword)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.Auth == nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	token, err := h.Auth.Issue()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.SessionSecure,
		MaxAge:   h.SessionMaxAge,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.SessionSecure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
