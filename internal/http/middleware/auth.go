package middleware

import (
	"net/http"
)

const SessionCookieName = "session_token"

type TokenValidator interface {
	Valid(token string) (bool, error)
}

func RequireAuth(auth TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth == nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			valid, err := auth.Valid(cookie.Value)
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
