// 26.01.31
// review
// 用于插值，单纯按照需要储存value的简易SQL

package sqlite

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/glebarez/go-sqlite" // SQLite 驱动
)

// Record 结构体用于表示从数据库中读取的记录
// 注意：这个结构体现在定义在 sqlite 包内部
type Record struct {
	Key   int64  `json:"key"`
	Value string `json:"value"`
}

// VSQLiteDB 是一个 SQLite 数据库包装器，用于键值存储
type VSQLiteDB struct {
	db        *sql.DB
	tableName string // 表名
}

// NewVSQLiteDB 初始化并返回一个新的 VSQLiteDB 实例
// dbPath 是 SQLite 数据库文件的路径
func NewVSQLiteDB(dbPath string, tableName string) (*VSQLiteDB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开数据库文件: %w", err)
	}

	// 尝试连接并验证数据库是否可用
	if err = db.Ping(); err != nil {
		db.Close() // 确保关闭连接
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	kvDB := &VSQLiteDB{
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
func (s *VSQLiteDB) DB() *sql.DB {
	return s.db
}

// Close 关闭数据库连接
func (s *VSQLiteDB) Close() error {
	if s.db != nil {
		// log.Println("关闭 SQLite 数据库连接...")
		return s.db.Close()
	}
	return nil
}

// CreateTable 在数据库中创建键值表
// 表名为 kv_store，包含 key (INTEGER PRIMARY KEY AUTOINCREMENT) 和 value (TEXT)
func (s *VSQLiteDB) CreateTable() error {
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			key INTEGER PRIMARY KEY AUTOINCREMENT,
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

// AddValue 将一个值添加到表中，并返回新生成的自增键。
func (s *VSQLiteDB) AddValue(value string) (int64, error) {
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (value) VALUES (?);
	`, s.tableName)

	result, err := s.db.Exec(insertSQL, value)
	if err != nil {
		return 0, fmt.Errorf("添加值失败 (value: %s): %w", value, err)
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取最后插入的ID失败: %w", err)
	}
	// log.Printf("已添加: key=%d, value='%s'", lastID, value)
	return lastID, nil
}

// QueryValue 根据给定的键查询其对应的值
// 如果键不存在，则返回空字符串和 sql.ErrNoRows 错误
func (s *VSQLiteDB) QueryValue(key int64) (string, error) {
	querySQL := fmt.Sprintf(`SELECT value FROM %s WHERE key = ?;`, s.tableName)

	var value string
	err := s.db.QueryRow(querySQL, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			// log.Printf("查询失败: 键 '%d' 不存在。", key)
			return "", err // 返回 sql.ErrNoRows 表示未找到
		}
		return "", fmt.Errorf("查询键 '%d' 失败: %w", key, err)
	}
	// log.Printf("查询成功: key='%d', value='%s'", key, value)
	return value, nil
}

// DeleteKey 根据给定的键删除键值对
// 如果键不存在，则返回 sql.ErrNoRows 错误。
func (s *VSQLiteDB) DeleteKey(key int64) error {
	deleteSQL := fmt.Sprintf(`DELETE FROM %s WHERE key = ?;`, s.tableName)
	result, err := s.db.Exec(deleteSQL, key)
	if err != nil {
		return fmt.Errorf("删除键 '%d' 失败: %w", key, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// log.Printf("删除操作: 键 '%d' 不存在。", key)
		return sql.ErrNoRows // 如果没有行受影响，表示键不存在
	} else {
		// log.Printf("删除成功: 键 '%d'。", key)
	}
	return nil
}

// CountRecords 返回表中的记录总数。
func (s *VSQLiteDB) CountRecords() (int64, error) {
	querySQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s;`, s.tableName)
	var count int64
	err := s.db.QueryRow(querySQL).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("获取记录总数失败: %w", err)
	}
	return count, nil
}

// MaxKey 返回表中最大的键值。
// 如果表为空，则返回 0 和 sql.ErrNoRows 错误。
func (s *VSQLiteDB) MaxKey() (int64, error) {
	querySQL := fmt.Sprintf(`SELECT MAX(key) FROM %s;`, s.tableName)
	var maxKey sql.NullInt64 // 使用 sql.NullInt64 来处理空表时 MAX 返回 NULL 的情况
	err := s.db.QueryRow(querySQL).Scan(&maxKey)
	if err != nil {
		return 0, fmt.Errorf("获取最大键失败: %w", err)
	}
	if !maxKey.Valid {
		// 表为空
		return 0, sql.ErrNoRows // 返回 sql.ErrNoRows 表示未找到任何键
	}
	return maxKey.Int64, nil
}

// GetRandomRecord 从表中随机读取一条记录。
// 如果表为空，则返回 sql.ErrNoRows 错误。
func (s *VSQLiteDB) GetRandomRecord() (Record, error) {
	querySQL := fmt.Sprintf(`SELECT key, value FROM %s ORDER BY RANDOM() LIMIT 1;`, s.tableName)
	var record Record
	err := s.db.QueryRow(querySQL).Scan(&record.Key, &record.Value)
	if err != nil {
		if err == sql.ErrNoRows {
			return Record{}, sql.ErrNoRows // 表中没有记录
		}
		return Record{}, fmt.Errorf("随机查询记录失败: %w", err)
	}
	return record, nil
}

/*
// Example usage (optional, for testing)
// 可以将以下代码块放到一个 main 函数中进行测试

func main() {
	dbPath := "test.db"
	tableName := "my_kv_store"
	kvDB, err := NewVSQLiteDB(dbPath, tableName) // 注意这里是 NewVSQLiteDB
	if err != nil {
		log.Fatalf("Failed to initialize DB: %v", err)
	}
	defer func() {
		if err := kvDB.Close(); err != nil {
			log.Printf("Error closing DB: %v", err)
		}
		// os.Remove(dbPath) // 清理测试数据库文件，谨慎使用
	}()


	// --- 测试 AddValue ---
	fmt.Println("\n--- Testing AddValue ---")
	id1, err := kvDB.AddValue("Hello World")
	if err != nil {
		log.Fatalf("Failed to add value: %v", err)
	}
	fmt.Printf("Added value with key: %d\n", id1)

	id2, err := kvDB.AddValue("Another value")
	if err != nil {
		log.Fatalf("Failed to add value: %v", err)
	}
	fmt.Printf("Added value with key: %d\n", id2)

	// --- 测试 QueryValue ---
	fmt.Println("\n--- Testing QueryValue ---")
	val, err := kvDB.QueryValue(id1)
	if err != nil {
		log.Fatalf("Failed to query value for key %d: %v", id1, err)
	}
	fmt.Printf("Queried key %d, value: %s\n", id1, val)

	// 查询不存在的键
	_, err = kvDB.QueryValue(999)
	if err == sql.ErrNoRows {
		fmt.Printf("Correctly reported key 999 not found.\n")
	} else if err != nil {
		log.Fatalf("Unexpected error when querying non-existent key: %v", err)
	}

	// --- 测试 CountRecords ---
	fmt.Println("\n--- Testing CountRecords ---")
	count, err := kvDB.CountRecords()
	if err != nil {
		log.Fatalf("Failed to count records: %v", err)
	}
	fmt.Printf("Total records: %d\n", count)

	// --- 测试 MaxKey ---
	fmt.Println("\n--- Testing MaxKey ---")
	max, err := kvDB.MaxKey()
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("Table is empty (MaxKey test, should not be empty now).")
		} else {
			log.Fatalf("Failed to get max key: %v", err)
		}
	} else {
		fmt.Printf("Max key: %d\n", max)
	}

	// --- 测试 GetRandomRecord ---
	fmt.Println("\n--- Testing GetRandomRecord ---")
	randomRecord, err := kvDB.GetRandomRecord()
	if err != nil {
		log.Fatalf("Failed to get random record: %v", err)
	}
	fmt.Printf("Random record: Key=%d, Value='%s'\n", randomRecord.Key, randomRecord.Value)

	// --- 测试 DeleteKey ---
	fmt.Println("\n--- Testing DeleteKey ---")
	err = kvDB.DeleteKey(id1)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("Key %d not found for deletion (unexpected).\n", id1)
		} else {
			log.Fatalf("Failed to delete key %d: %v", id1, err)
		}
	} else {
		fmt.Printf("Deleted key: %d\n", id1)
	}

	// 尝试删除一个不存在的键
	err = kvDB.DeleteKey(999)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("Correctly reported key 999 not found for deletion.\n")
		} else {
			log.Fatalf("Unexpected error when deleting non-existent key: %v", err)
		}
	}

	// --- 再次测试 CountRecords after deletion ---
	fmt.Println("\n--- Testing CountRecords after deletion ---")
	count, err = kvDB.CountRecords()
	if err != nil {
		log.Fatalf("Failed to count records after deletion: %v", err)
	}
	fmt.Printf("Total records after deletion: %d\n", count)

	// 清空表并测试 MaxKey on empty table
	fmt.Println("\n--- Testing MaxKey on empty table ---")
	err = kvDB.DeleteKey(id2) // Delete the last remaining key
	if err != nil {
		log.Fatalf("Failed to delete last key: %v", err)
	}

	max, err = kvDB.MaxKey()
	if err == sql.ErrNoRows {
		fmt.Printf("Correctly reported table is empty (MaxKey returned 0 and sql.ErrNoRows).\n")
	} else if err != nil {
		log.Fatalf("Unexpected error when getting max key from empty table: %v", err)
	} else {
		fmt.Printf("Max key on empty table: %d (unexpected, should be 0 and error)\n", max)
	}

	// 测试 GetRandomRecord on empty table
	fmt.Println("\n--- Testing GetRandomRecord on empty table ---")
	_, err = kvDB.GetRandomRecord()
	if err == sql.ErrNoRows {
		fmt.Printf("Correctly reported table is empty (GetRandomRecord returned sql.ErrNoRows).\n")
	} else if err != nil {
		log.Fatalf("Unexpected error when getting random record from empty table: %v", err)
	}
}
*/
