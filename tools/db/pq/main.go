// usage:
// 把一个只返回err的handler function传进来。
// TODO:
// 这里db每次open close肯定不对的。

package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() {
	db, err := ConnectPostgreSQL("localhost", 5432, os.Getenv("PSQL_USER"), os.Getenv("PSQL_PASSWORD"), os.Getenv("PSQL_DBNAME"))
	if err != nil {
		panic(err)
	}
	DB = db
}

func Exec(handler func(tx *sql.Tx) error) error {
	// connStr := os.Getenv("CONN_STR")
	// db, err := sql.Open("postgres", connStr)
	// if err != nil {
	// return err
	// }
	// defer db.Close()
	db := DB

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// 如果handler当中不Commit那么就会统一Rollback
	defer tx.Rollback()

	return handler(tx)
}

// 封装连接函数
func ConnectPostgreSQL(host string, port int, user, password, dbname string) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: %v", err)
	}

	// 配置连接池
	db.SetMaxIdleConns(10)  // 最大空闲连接
	db.SetMaxOpenConns(100) // 最大活跃连接

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库心跳检测失败: %v", err)
	}
	return db, nil
}
