package store

import "database/sql"

// D1Store implements Store on top of Cloudflare D1 via Go's database/sql driver.
// Connections are opened lazily per operation so the Cloudflare Workers runtime
// request context (env.DB) is always active when queries run.
type D1Store struct {
	openDB func() (*sql.DB, error)
}

// NewD1Store creates a D1-backed Store using the given sql.DB opener.
func NewD1Store(openDB func() (*sql.DB, error)) *D1Store {
	return &D1Store{openDB: openDB}
}

func (s *D1Store) BackendName() string { return "cloudflare-d1" }

func (s *D1Store) withDB(fn func(db *sql.DB) error) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(db)
}

type rowScanner interface {
	Scan(dest ...any) error
}
