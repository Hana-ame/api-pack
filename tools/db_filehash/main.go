package db

import (
	"database/sql"

	_ "github.com/glebarez/go-sqlite"

	// _ "github.com/mattn/go-sqlite3"
	"log"
)

var DB *sql.DB

func init() {
	db, err := sql.Open("sqlite", "./foo.db")
	if err != nil {
		log.Fatal(err)
	}
	// defer db.Close()

	sqlStmt := `
CREATE TABLE IF NOT EXISTS file_hashes (
	id INTEGER PRIMARY KEY,
	path TEXT NOT NULL,
	sha1_sum TEXT NOT NULL
);`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}

	DB = db
}

func CreatePathHash(path, hash string) {
	sqlStmt := `
INSERT INTO file_hashes (path, sha1_sum)
VALUES (?, ?)
;`
	_, err := DB.Exec(sqlStmt, path, hash)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}
}

func ReadPathByHash(hash string) (string, error) {
	sqlStmt := `
SELECT path
FROM file_hashes
WHERE sha1_sum = ?
;`
	row := DB.QueryRow(sqlStmt, hash)
	var path sql.NullString
	err := row.Scan(&path)

	return path.String, err
}

func DeleteByPath(path string) error {
	sqlStmt := `
DELETE FROM file_hashes
WHERE path = ?
;`
	_, err := DB.Exec(sqlStmt, path)
	return err
}
