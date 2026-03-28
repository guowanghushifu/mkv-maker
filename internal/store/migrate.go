package store

import "database/sql"

func Migrate(db *sql.DB) error {
	stmts := []string{
		`create table if not exists sessions (
			token text primary key,
			remote_addr text not null,
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
	return nil
}
