package main

import (
	"fmt"

	"github.com/Hana-ame/api-pack/tools/sqlite"
	_ "github.com/glebarez/go-sqlite"
)

func main() {
	db, err := sqlite.NewKVSQLiteDB("kv.db", "query_value")
	fmt.Println(db)
	fmt.Println(err)
}
