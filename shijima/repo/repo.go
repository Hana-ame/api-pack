package repo

import "database/sql"

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }
func (r *Repo) DB() *sql.DB { return r.db }
