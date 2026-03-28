package store

import "database/sql"

func Migrate(db *sql.DB) error {
	stmts := []string{
		`create table if not exists sessions (
			token text primary key,
			remote_addr text not null,
			expires_at datetime not null,
			created_at datetime not null default current_timestamp
		);`,
		`create table if not exists jobs (
			id text primary key,
			status text not null,
			draft_json text not null,
			output_path text not null default '',
			log_path text not null default '',
			error_text text not null default '',
			created_at datetime not null default current_timestamp,
			started_at datetime,
			finished_at datetime
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := ensureSessionsExpiryColumn(db); err != nil {
		return err
	}
	return nil
}

func ensureSessionsExpiryColumn(db *sql.DB) error {
	rows, err := db.Query(`pragma table_info(sessions)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasExpiresAt := false
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return err
		}
		if name == "expires_at" {
			hasExpiresAt = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasExpiresAt {
		return nil
	}

	if _, err := db.Exec(`alter table sessions add column expires_at datetime`); err != nil {
		return err
	}
	if _, err := db.Exec(`update sessions set expires_at = datetime(created_at, '+1 day') where expires_at is null`); err != nil {
		return err
	}
	return nil
}
