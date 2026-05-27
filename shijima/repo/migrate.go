package repo

import "fmt"

func (r *Repo) InitDB() error {
	if _, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("initDB: %w", err)
	}

	var version int
	r.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)

	migrations := []struct {
		v   int
		ddl []string
	}{
		{1, []string{
			`CREATE TABLE IF NOT EXISTS thread (
				no INTEGER PRIMARY KEY AUTOINCREMENT,
				t TEXT DEFAULT '',
				n TEXT DEFAULT '',
				ts TEXT DEFAULT (datetime('now')),
				id TEXT DEFAULT '',
				p TEXT DEFAULT '',
				txt TEXT DEFAULT '',
				r INTEGER DEFAULT 0,
				del INTEGER DEFAULT 0,
				c TEXT DEFAULT '',
				ip TEXT DEFAULT ''
			)`,
			`CREATE TABLE IF NOT EXISTS board (
				bid INTEGER NOT NULL,
				tid INTEGER NOT NULL,
				replynum INTEGER DEFAULT 0,
				last TEXT DEFAULT (datetime('now')),
				PRIMARY KEY (bid, tid)
			)`,
			`CREATE TABLE IF NOT EXISTS reactions_alt (
				tid INTEGER NOT NULL,
				reaction TEXT NOT NULL,
				count INTEGER NOT NULL DEFAULT 0,
				timestamp TEXT DEFAULT (datetime('now')),
				PRIMARY KEY (tid, reaction)
			)`,
		}},
		{2, []string{
			`CREATE TABLE IF NOT EXISTS channel (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL DEFAULT '',
				description TEXT NOT NULL DEFAULT '',
				mode TEXT NOT NULL DEFAULT 'chat',
				created_at TEXT NOT NULL DEFAULT (datetime('now'))
			)`,
			`CREATE TABLE IF NOT EXISTS message (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				channel_id INTEGER NOT NULL,
				parent_id INTEGER NOT NULL DEFAULT 0,
				author_id TEXT NOT NULL DEFAULT '',
				title TEXT NOT NULL DEFAULT '',
				author_name TEXT NOT NULL DEFAULT '',
				content TEXT NOT NULL DEFAULT '',
				image TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT (datetime('now')),
				edited_at TEXT NOT NULL DEFAULT '',
				deleted INTEGER NOT NULL DEFAULT 0,
				country TEXT NOT NULL DEFAULT '',
				ip TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE TABLE IF NOT EXISTS reaction (
				message_id INTEGER NOT NULL,
				emoji TEXT NOT NULL,
				count INTEGER DEFAULT 0,
				updated_at TEXT NOT NULL DEFAULT (datetime('now')),
				PRIMARY KEY (message_id, emoji)
			)`,
		}},
	}

	for _, m := range migrations {
		if m.v > version {
			for _, ddl := range m.ddl {
				if _, err := r.db.Exec(ddl); err != nil {
					return fmt.Errorf("migration v%d: %w", m.v, err)
				}
			}
			r.db.Exec(`INSERT INTO schema_version (version) VALUES (?)`, m.v)
		}
	}
	return nil
}

