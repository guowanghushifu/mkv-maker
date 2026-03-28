package middleware

import (
	"net/http"

	"github.com/wangdazhuo/mkv-maker/internal/store"
)

const SessionCookieName = "session_token"

func RequireAuth(sessions *store.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sessions == nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			valid, err := sessions.Valid(cookie.Value)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if !valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
