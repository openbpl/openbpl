package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS detections (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	domain    TEXT    NOT NULL,
	keyword   TEXT    NOT NULL,
	kind      TEXT    NOT NULL,
	distance  INTEGER NOT NULL DEFAULT 0,
	seen_at   DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_detections_domain ON detections (domain);
`

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func (s *DB) Close() error {
	return s.db.Close()
}

type Detection struct {
	Domain   string
	Keyword  string
	Kind     string
	Distance int
	SeenAt   time.Time
}

func (s *DB) Insert(d Detection) error {
	_, err := s.db.Exec(
		`INSERT INTO detections (domain, keyword, kind, distance, seen_at) VALUES (?, ?, ?, ?, ?)`,
		d.Domain, d.Keyword, d.Kind, d.Distance, d.SeenAt,
	)
	return err
}
