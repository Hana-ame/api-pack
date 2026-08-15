package handler

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	_ "github.com/joho/godotenv/autoload"
	"github.com/stretchr/testify/assert"

	_ "github.com/lib/pq" // 导入 PostgreSQL 驱动
)

// connStr 是数据库连接字符串，可以从环境变量中获取
var connStr = os.Getenv("CONN_STR")

// setupDB 初始化数据库连接并返回一个事务
func setupDB(t *testing.T) (*sql.DB, *sql.Tx) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	// 开始一个事务
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	return db, tx
}

// teardownDB 清理数据库，回滚事务并关闭连接
func teardownDB(t *testing.T, db *sql.DB, tx *sql.Tx) {
	if err := tx.Rollback(); err != nil {
		t.Fatalf("failed to rollback transaction: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close database connection: %v", err)
	}
}

func TestReadFileMIMEType(t *testing.T) {
	// 初始化数据库连接和事务
	db, tx := setupDB(t)
	defer teardownDB(t, db, tx)

	// 插入测试数据
	_, err := tx.Exec("INSERT INTO files (id, mime_type) VALUES ($1, $2)", 1, "image/png")
	assert.NoError(t, err, "failed to insert test data")

	// 调用被测试的函数
	mimeType, err := ReadFileMIMEType(tx, 1)
	assert.NoError(t, err, "failed to read MIME type")
	assert.Equal(t, "image/png", mimeType, "MIME type should be 'image/png'")
}

func TestCreateFileMIMEType(t *testing.T) {
	// 初始化数据库连接和事务
	db, tx := setupDB(t)
	defer teardownDB(t, db, tx)

	// 调用被测试的函数
	err := CreateFileMIMEType(tx, 2, "application/pdf")
	assert.NoError(t, err, "failed to create MIME type")

	// 验证数据是否插入成功
	var mimeType string
	err = tx.QueryRow("SELECT mime_type FROM files WHERE id = $1", 2).Scan(&mimeType)
	assert.NoError(t, err, "failed to query inserted data")
	assert.Equal(t, "application/pdf", mimeType, "MIME type should be 'application/pdf'")
}

func TestFiles(t *testing.T) {
	db, tx := setupDB(t)
	defer teardownDB(t, db, tx)

	id, err := ReadFiles(tx, tools.NewSlice("image/png"))
	fmt.Println(id, err)
}

func TestFilesBefore(t *testing.T) {
	db, tx := setupDB(t)
	defer teardownDB(t, db, tx)

	id, err := ReadFilesBefore(tx, 114049881453953024, tools.NewSlice("image/png"))
	fmt.Println(id, err)
}
