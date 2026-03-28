package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
)

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(remoteAddr string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	_, err := s.db.Exec(`insert into sessions(token, remote_addr) values(?, ?)`, token, remoteAddr)
	return token, err
}

func (s *SessionStore) Valid(token string) bool {
	var count int
	_ = s.db.QueryRow(`select count(1) from sessions where token = ?`, token).Scan(&count)
	return count == 1
}
