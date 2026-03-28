package middleware

import (
	"net/http"

	"github.com/wangdazhuo/mkv-maker/internal/store"
)

const sessionCookieName = "session_token"

func RequireAuth(sessions *store.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || !sessions.Valid(cookie.Value) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
