package db

import (
	// _ "github.com/Hana-ame/api-pack/tools/db_filehash"
	"database/sql"
	"fmt"
	"log"
	"testing"
)

func Init() {
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
);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}

	DB = db

}
func TestRead(t *testing.T) {
	Init()
	a, e := ReadPathByHash("12312")
	fmt.Println(a, e)
	fmt.Println("=====")
	t.Error(e)
}
