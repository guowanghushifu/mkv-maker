package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/wangdazhuo/mkv-maker/internal/store"
)

const SessionCookieName = "session_token"

type AuthHandler struct {
	AppPassword   string
	Sessions      *store.SessionStore
	SessionMaxAge int
}

type loginRequest struct {
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.AppPassword)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	token, err := h.Sessions.Create(r.RemoteAddr)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   h.SessionMaxAge,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
