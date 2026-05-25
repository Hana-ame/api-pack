package shijima

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Hana-ame/api-pack/utils/orderedmap"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
)

// createTableIfNotExists 用于创建 reactions_alt 表（如果不存在）
func createTableIfNotExists() error {
	query := `
		CREATE TABLE IF NOT EXISTS reactions_alt (
			tid INTEGER NOT NULL,
			reaction TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			timestamp TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (tid, reaction)
		);`
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table reactions_alt: %w", err)
	}
	fmt.Println("Table reactions_alt ensured to exist.")
	return nil
}

// setReactionAlt 函数将 tid 和 reaction 的 count 增加1。
// 如果 (tid, reaction) 组合不存在，则会插入新记录，count 为 1。
// 如果存在，则会更新现有记录，count 增加 1，并且 timestamp 会自动更新。
func setReactionAlt(tid int, reaction string) error {
	query := `
			INSERT INTO reactions_alt (tid, reaction, count, timestamp)
			VALUES (?, ?, 1, datetime('now'))
			ON CONFLICT(tid, reaction) DO UPDATE SET
				count = count + 1,
				timestamp = datetime('now');
		`
	_, err := db.Exec(query, tid, []byte(reaction))
	if err != nil {
		return fmt.Errorf("failed to set reaction for tid %d, reaction %s: %w", tid, reaction, err)
	}
	return nil
}

func getReactionsAlt(tid int) (*orderedmap.OrderedMap, error) {
	query := `
			SELECT reaction, count
			FROM reactions_alt
			WHERE tid = ?
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
