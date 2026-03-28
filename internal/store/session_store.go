package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type SessionStore struct {
	db                   *sql.DB
	sessionMaxAgeSeconds int
}

func NewSessionStore(db *sql.DB, sessionMaxAge time.Duration) *SessionStore {
	maxAgeSeconds := int(sessionMaxAge / time.Second)
	if maxAgeSeconds < 0 {
		maxAgeSeconds = 0
	}
	return &SessionStore{
		db:                   db,
		sessionMaxAgeSeconds: maxAgeSeconds,
	}
}

func (s *SessionStore) Create(remoteAddr string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	expiresModifier := fmt.Sprintf("+%d seconds", s.sessionMaxAgeSeconds)
	_, err := s.db.Exec(
		`insert into sessions(token, remote_addr, expires_at) values(?, ?, datetime('now', ?))`,
		token,
		remoteAddr,
		expiresModifier,
	)
	return token, err
}

func (s *SessionStore) Valid(token string) (bool, error) {
	if token == "" {
		return false, nil
	}

	var count int
	if err := s.db.QueryRow(
		`select count(1) from sessions where token = ? and datetime(expires_at) > current_timestamp`,
		token,
	).Scan(&count); err != nil {
		return false, err
	}
	return count == 1, nil
}
