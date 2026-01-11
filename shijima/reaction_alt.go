package shijima

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // PostgreSQL driver (Changed from mysql)
)

// createTableIfNotExists 用于创建 reactions_alt 表
func createTableIfNotExists() error {
	// PostgreSQL:
	// 1. BIGINT(20) -> BIGINT
	// 2. ON UPDATE CURRENT_TIMESTAMP 属性在 PG 中不存在，通常通过触发器或在查询中手动设置
	query := `
	CREATE TABLE IF NOT EXISTS reactions_alt (
		tid BIGINT NOT NULL,
		reaction VARCHAR(100) NOT NULL,
		count BIGINT NOT NULL DEFAULT 0,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (tid, reaction)
	);`
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table reactions_alt: %w", err)
	}
	fmt.Println("Table reactions_alt ensured to exist in PostgreSQL.")
	return nil
}

// setReactionAlt 增加计数 (Upsert)
func setReactionAlt(tid int, reaction string) error {
	// PostgreSQL 使用 ON CONFLICT 子句处理冲突
	// 使用 $1, $2 替代 ?
	// 在 UPDATE SET 中明确指定 timestamp = CURRENT_TIMESTAMP 以模拟 MySQL 的自动更新行为
	query := `
		INSERT INTO reactions_alt (tid, reaction, count)
		VALUES ($1, $2, 1)
		ON CONFLICT (tid, reaction) 
		DO UPDATE SET 
			count = reactions_alt.count + 1, 
			timestamp = CURRENT_TIMESTAMP;
	`
	_, err := db.Exec(query, tid, reaction) // PG 不需要像 MySQL 那样强转 []byte
	if err != nil {
		return fmt.Errorf("failed to set reaction for tid %d, reaction %s: %w", tid, reaction, err)
	}
	return nil
}

// getReactionsAlt 获取统计结果
func getReactionsAlt(tid int) (*orderedmap.OrderedMap, error) {
	// 更改占位符为 $1
	query := `
		SELECT reaction, count
		FROM reactions_alt
		WHERE tid = $1
		ORDER BY count DESC, timestamp ASC;
	`

	rows, err := db.Query(query, tid)
	if err != nil {
		return nil, fmt.Errorf("failed to query reactions for tid %d: %w", tid, err)
	}
	defer rows.Close()

	om := orderedmap.NewOrderedMap()

	for rows.Next() {
		var reaction string
		var count int
		if err := rows.Scan(&reaction, &count); err != nil {
			return nil, fmt.Errorf("failed to scan reaction row for tid %d: %w", tid, err)
		}
		om.Set(reaction, count)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for tid %d: %w", tid, err)
	}

	return om, nil
}

// GetReactionsHandlerAlt 处理获取请求 (保持不变)
func GetReactionsHandlerAlt(c *gin.Context) {
	tidStr := c.Param("tid")
	tid, err := strconv.Atoi(tidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的tid参数"})
		return
	}

	reactionCountsMap, err := getReactionsAlt(tid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取reactions失败: %v", err)})
		return
	}

	response := gin.H{
		"reactions":   reactionCountsMap,
		"my_reaction": "",
	}

	c.JSON(http.StatusOK, response)
}

// SetReactionHandlerAlt 处理设置请求 (保持不变)
func SetReactionHandlerAlt(c *gin.Context) {
	tidStr := c.Param("tid")
	tid, err := strconv.Atoi(tidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的tid参数"})
		c.Abort()
		return
	}

	reaction := string(tools.Match(c.GetRawData()).Result())

	err = setReactionAlt(tid, reaction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("设置reaction失败: %v", err)})
		c.Abort()
		return
	}

	message := "Reaction操作成功"
	if reaction == "" || reaction == "none" {
		message = "Reaction已移除"
	} else {
		message = fmt.Sprintf("Reaction已设置为: %s", reaction)
	}

	if reaction == "🎲" {
		postThread(&Thread{
			ID:  c.GetString("id"),
			R:   uint(tid),
			Txt: "自动roll点 [1,100]\n@rd 1d100",
		}, 0)
	}

	c.JSON(http.StatusOK, gin.H{"message": message})
}
