package bot

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

var _db *BotDB

func SetDB(db *BotDB) {
	_db = db
}

// Record 结构体用于表示从数据库中读取的记录
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

// NewDB initializes and returns a BotDB instance.
// It does not create the table itself, assuming it already exists.
// If you wanted to create the table, you'd add db.Exec here.
func NewDB(db *sql.DB) (*BotDB, error) {
	// Optional: Ping the database to ensure connection is live
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	// log.Println("Database connection established.")
	return &BotDB{
		db: db,
	}, nil
}

func (db *BotDB) DB() *sql.DB {
	return db.db
}

// GetRecord fetches a single record by its composite primary key (tid, bot, query).
// Returns sql.ErrNoRows if no matching record is found.
func (b *BotDB) GetRecord(tid int64, bot string, query string) (*Record, error) {
	record := &Record{}
	err := b.db.QueryRow(
		"SELECT tid, bot, query, status, response FROM bot WHERE tid = ? AND bot = ? AND query = ?",
		tid, bot, query,
	).Scan(
		&record.TID,
		&record.Bot,
		&record.Query,
		&record.Status,
		&record.Response,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No record found, return nil record and nil error
		}
		return nil, fmt.Errorf("failed to query record by primary key: %w", err)
	}

	return record, nil
}

// GetRecordsByTID fetches all records associated with a specific TID.
// Returns an empty slice if no records are found for the given tid.
func (b *BotDB) GetRecordsByTID(tid int64) ([]Record, error) {
	rows, err := b.db.Query(
		"SELECT tid, bot, query, status, response FROM bot WHERE tid = ?",
		tid,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query records by tid: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		if err := rows.Scan(
			&record.TID,
			&record.Bot,
			&record.Query,
			&record.Status,
			&record.Response,
		); err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}
		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating rows: %w", err)
	}

	return records, nil
}

// InsertOrUpdate inserts a new record or updates an existing one
// based on the composite primary key (tid, bot, query).
func (b *BotDB) InsertOrUpdate(tid int64, bot string, query string, status string, response string) error {
	// Using INSERT ... ON DUPLICATE KEY UPDATE is the standard way to achieve this in MySQL.
	// VALUES(column_name) refers to the value provided in the INSERT clause for that column.
	stmt := `
		INSERT INTO bot (tid, bot, query, status, response)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			status = VALUES(status),
			response = VALUES(response);
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
	return InsertOrUpdate(tid, bot, query, status, response)
}
