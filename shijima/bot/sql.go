package bot

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

var _db *BotDB

func SetDB(db *BotDB) {
	_db = db
}

type Record struct {
	TID      int64  `json:"tid"`
	Bot      string `json:"bot"`
	Query    string `json:"query"`
	Status   string `json:"status"`
	Response string `json:"response"`
}

type BotDB struct {
	db *sql.DB
}

func NewDB(db *sql.DB) (*BotDB, error) {
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return &BotDB{db: db}, nil
}

func (b *BotDB) DB() *sql.DB {
	return b.db
}

func (b *BotDB) GetRecord(tid int64, bot string, query string) (*Record, error) {
	record := &Record{}
	err := b.db.QueryRow(
		"SELECT tid, bot, query, status, response FROM bot WHERE tid = ? AND bot = ? AND query = ?",
		tid, bot, query,
	).Scan(
		&record.TID, &record.Bot, &record.Query, &record.Status, &record.Response,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query record by primary key: %w", err)
	}
	return record, nil
}

func (b *BotDB) GetRecordsByTID(tid int64) ([]Record, error) {
	rows, err := b.db.Query(
		"SELECT tid, bot, query, status, response FROM bot WHERE tid = ?", tid,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query records by tid: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		if err := rows.Scan(&record.TID, &record.Bot, &record.Query, &record.Status, &record.Response); err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}
		records = append(records, record)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating rows: %w", err)
	}
	return records, nil
}

func (b *BotDB) InsertOrUpdate(tid int64, bot string, query string, status string, response string) error {
	stmt := `
		INSERT INTO bot (tid, bot, query, status, response)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(tid, bot, query) DO UPDATE SET
			status = excluded.status,
			response = excluded.response;
	`
	_, err := b.db.Exec(stmt, tid, bot, query, status, response)
	if err != nil {
		return fmt.Errorf("failed to insert or update record: %w", err)
	}
	return nil
}

func DB() *sql.DB {
	return _db.DB()
}
func GetRecord(tid int64, bot string, query string) (*Record, error) {
	return _db.GetRecord(tid, bot, query)
}
func GetRecordsByTID(tid int64) ([]Record, error) {
	return _db.GetRecordsByTID(tid)
}
func InsertOrUpdate(tid int64, bot string, query string, status string, response string) error {
	return _db.InsertOrUpdate(tid, bot, query, status, response)
}
