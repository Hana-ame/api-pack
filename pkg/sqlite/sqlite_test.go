package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"testing"
)

func TestAAA(t *testing.T) {
	dbFileName := "my_kv_store.db"
	tableName := "kv_data"

	// 1. 初始化数据库包装器
	kvDB, err := NewKVSQLiteDB(dbFileName, tableName)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer func() {
		if err := kvDB.Close(); err != nil {
			// log.Printf("关闭数据库时发生错误: %v", err)
		}
		// 可以在这里选择删除数据库文件，方便测试
		// os.Remove(dbFileName)
	}()

	// 2. 添加/更新数据
	log.Println("\n--- 添加/更新数据 ---")
	kvDB.AddOrUpdate("name", "Alice")
	kvDB.AddOrUpdate("age", "30")
	kvDB.AddOrUpdate("city", "New York")
	kvDB.AddOrUpdate("name", "Bob") // 更新 'name' 的值

	// 3. 查询数据
	log.Println("\n--- 查询数据 ---")
	value, err := kvDB.QueryValue("name")
	if err != nil && err != sql.ErrNoRows {
		log.Printf("查询 'name' 失败: %v", err)
	} else if err == nil {
		fmt.Printf("Key 'name' 的值为: %s\n", value)
	}

	value, err = kvDB.QueryValue("age")
	if err != nil && err != sql.ErrNoRows {
		log.Printf("查询 'age' 失败: %v", err)
	} else if err == nil {
		fmt.Printf("Key 'age' 的值为: %s\n", value)
	}

	value, err = kvDB.QueryValue("country") // 查询一个不存在的键
	if err != nil && err == sql.ErrNoRows {
		fmt.Printf("Key 'country' 不存在。\n")
	} else if err != nil {
		log.Printf("查询 'country' 失败: %v", err)
	}

	// 4. 删除数据
	log.Println("\n--- 删除数据 ---")
	err = kvDB.DeleteKey("age")
	if err != nil {
		log.Printf("删除 'age' 失败: %v", err)
	}

	// 再次查询已删除的键
	value, err = kvDB.QueryValue("age")
	if err != nil && err == sql.ErrNoRows {
		fmt.Printf("再次查询 'age': 键已删除。\n")
	} else if err != nil {
		log.Printf("再次查询 'age' 失败: %v", err)
	}

	// 删除一个不存在的键
	err = kvDB.DeleteKey("non_existent_key")
	if err != nil {
		log.Printf("删除 'non_existent_key' 失败: %v", err)
	}
}
