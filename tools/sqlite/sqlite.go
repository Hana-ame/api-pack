// 26.01.31
// review
// 简易的插值查值KV表

package sqlite

import (
	"database/sql"
	"fmt"
	"log"

	// _ "github.com/mattn/go-sqlite3" // SQLite 驱动
	_ "github.com/glebarez/go-sqlite"
)

func NewSQLiteDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开数据库文件: %w", err)
	}

	// 尝试连接并验证数据库是否可用
	if err = db.Ping(); err != nil {
		db.Close() // 确保关闭连接
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	return db, nil
}

// KVSQLiteDB 是一个 SQLite 数据库包装器，用于键值存储
type KVSQLiteDB struct {
	db        *sql.DB
	tableName string // 表名
}

// NewKVSQLiteDB 初始化并返回一个新的 KVSQLiteDB 实例
// dbPath 是 SQLite 数据库文件的路径
func NewKVSQLiteDB(dbPath string, tableName string) (*KVSQLiteDB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开数据库文件: %w", err)
	}

	// 尝试连接并验证数据库是否可用
	if err = db.Ping(); err != nil {
		db.Close() // 确保关闭连接
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	kvDB := &KVSQLiteDB{
		db:        db,
		tableName: tableName,
	}

	// 自动创建表
	if err := kvDB.CreateTable(); err != nil {
		kvDB.Close() // 如果创建表失败，关闭数据库
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	// log.Printf("成功连接到 SQLite 数据库: %s", dbPath)
	return kvDB, nil
}

// Close 关闭数据库连接
func (s *KVSQLiteDB) Close() error {
	if s.db != nil {
		// log.Println("关闭 SQLite 数据库连接...")
		return s.db.Close()
	}
	return nil
}

// CreateTable 在数据库中创建键值表
// 表名为 kv_store，包含 key (TEXT PRIMARY KEY) 和 value (TEXT)
func (s *KVSQLiteDB) CreateTable() error {
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`, s.tableName)

	_, err := s.db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("执行创建表语句失败: %w", err)
	}
	log.Printf("表 '%s' 已就绪或已创建。", s.tableName)
	return nil
}

// AddOrUpdate 将键值对添加到表中。
// 如果键已存在，则更新其值；否则，插入新的键值对。
func (s *KVSQLiteDB) AddOrUpdate(key, value string) error {
	// ON CONFLICT(key) DO UPDATE SET value=excluded.value; 是 SQLite 3.24+ 的 Upsert 语法
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value;
	`, s.tableName)

	_, err := s.db.Exec(insertSQL, key, value)
	if err != nil {
		return fmt.Errorf("添加/更新键值对失败 (key: %s, value: %s): %w", key, value, err)
	}
	// log.Printf("已添加/更新: key='%s', value='%s'", key, value)
	return nil
}

// QueryValue 根据给定的键查询其对应的值
// 如果键不存在，则返回空字符串和 sql.ErrNoRows 错误
func (s *KVSQLiteDB) QueryValue(key string) (string, error) {
	querySQL := fmt.Sprintf(`SELECT value FROM %s WHERE key = ?;`, s.tableName)

	var value string
	err := s.db.QueryRow(querySQL, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			// log.Printf("查询失败: 键 '%s' 不存在。", key)
			return "", err // 返回 sql.ErrNoRows 表示未找到
		}
		return "", fmt.Errorf("查询键 '%s' 失败: %w", key, err)
	}
	// log.Printf("查询成功: key='%s', value='%s'", key, value)
	return value, nil
}

// DeleteKey 根据给定的键删除键值对
func (s *KVSQLiteDB) DeleteKey(key string) error {
	deleteSQL := fmt.Sprintf(`DELETE FROM %s WHERE key = ?;`, s.tableName)
	result, err := s.db.Exec(deleteSQL, key)
	if err != nil {
		return fmt.Errorf("删除键 '%s' 失败: %w", key, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// log.Printf("删除操作: 键 '%s' 不存在。", key)
	} else {
		// log.Printf("删除成功: 键 '%s'。", key)
	}
	return nil
}
