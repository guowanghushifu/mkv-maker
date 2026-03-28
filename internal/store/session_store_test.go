package store

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSessionStoreCreatesAndValidatesSession(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db, time.Hour)

	token, err := store.Create("127.0.0.1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	ok, err := store.Valid(token)
	if err != nil {
		t.Fatalf("Valid returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected created session token to validate")
	}
}

func TestSessionStoreRejectsExpiredSession(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db, 0)

	token, err := store.Create("127.0.0.1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	ok, err := store.Valid(token)
	if err != nil {
		t.Fatalf("Valid returned error: %v", err)
	}
	if ok {
		t.Fatal("expected session to be expired")
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return db
}
