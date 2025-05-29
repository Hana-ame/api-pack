package shijima

import (
	"fmt"
	"net/http"
	"strconv"

	// "github.com/elliotchance/orderedmap/v2" // 尼玛，没json的。

	tools "github.com/Hana-ame/api-pack/Tools"
	"github.com/Hana-ame/api-pack/Tools/orderedmap"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

// // db 数据库连接实例，通常在应用程序启动时初始化一次
// var db *sql.DB

// // initDB 用于初始化数据库连接
// func initDB() error {
// 	// 替换为你的数据库连接字符串
// 	// 格式： "user:password@tcp(host:port)/database_name?charset=utf8mb4&parseTime=True&loc=Local"
// 	// parseTime=True 是必需的，以便将MySQL的TIME、DATE、DATETIME类型正确解析为Go的time.Time
// 	// loc=Local 确保时间是本地时间，而不是UTC
// 	dsn := "root:password@tcp(127.0.0.1:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"

// 	var err error
// 	db, err = sql.Open("mysql", dsn)
// 	if err != nil {
// 		return fmt.Errorf("failed to open database connection: %w", err)
// 	}

// 	// 尝试ping数据库以验证连接
// 	err = db.Ping()
// 	if err != nil {
// 		return fmt.Errorf("failed to ping database: %w", err)
// 	}

// 	fmt.Println("Database connected successfully!")
// 	return nil
// }

// createTableIfNotExists 用于创建 reactions_alt 表（如果不存在）
// 假设 tid 和 reaction 组合是主键，并且 timestamp 默认是当前时间戳，并在更新时自动更新
func createTableIfNotExists() error {
	query := `
	CREATE TABLE IF NOT EXISTS reactions_alt (
		tid BIGINT(20) NOT NULL,
		reaction VARCHAR(100) NOT NULL,
		count BIGINT(20) NOT NULL DEFAULT 0,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
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
	// 使用 INSERT ... ON DUPLICATE KEY UPDATE 语句来处理 upsert 逻辑
	// 如果 tid 和 reaction 组合已存在（主键冲突），则执行 UPDATE 部分
	// count = count + 1 增加计数
	// timestamp = CURRENT_TIMESTAMP 确保时间戳更新到最新
	query := `
		INSERT INTO reactions_alt (tid, reaction, count)
		VALUES (?, ?, 1)
		ON DUPLICATE KEY UPDATE count = count + 1, timestamp = CURRENT_TIMESTAMP;
	`
	_, err := db.Exec(query, tid, []byte(reaction))
	if err != nil {
		return fmt.Errorf("failed to set reaction for tid %d, reaction %s: %w", tid, reaction, err)
	}
	return nil
}

// getReactions 函数根据 tid 查询 reactions，并返回一个有序映射。
// 映射中的键是 reaction 字符串，值是对应的 count。
// 结果会按照 count 降序排列，如果 count 相同，则按 timestamp 降序排列。
func getReactionsAlt(tid int) (*orderedmap.OrderedMap, error) {
	// 查询语句：选择 reaction 和 count，根据 tid 过滤
	// ORDER BY count DESC, timestamp DESC 实现排序需求
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
	defer rows.Close() // 确保在函数退出时关闭行集

	// 创建一个新的 orderedmap 实例
	om := orderedmap.NewOrderedMap()

	// 遍历查询结果
	for rows.Next() {
		var reaction string
		var count int
		// 扫描每一行的结果到变量
		if err := rows.Scan(&reaction, &count); err != nil {
			return nil, fmt.Errorf("failed to scan reaction row for tid %d: %w", tid, err)
		}
		// 将结果按顺序添加到有序映射中
		// 因为SQL查询已经排序，所以直接添加即可
		om.Set(reaction, count)
	}

	// 检查遍历过程中是否有错误发生
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for tid %d: %w", tid, err)
	}

	return om, nil
}

// GetReactionsHandler 处理获取帖子 reactions 的 HTTP 请求
// GET /reactions/:tid
func GetReactionsHandlerAlt(c *gin.Context) {
	tidStr := c.Param("tid")
	tid, err := strconv.Atoi(tidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的tid参数"})
		return
	}

	// myIP := c.GetHeader("X-Forwarded-For") // 获取客户端IP

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

// SetReactionHandler 处理设置/更新/取消用户 reaction 的 HTTP 请求
// POST /reactions/:tid
func SetReactionHandlerAlt(c *gin.Context) {
	tidStr := c.Param("tid")
	tid, err := strconv.Atoi(tidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的tid参数"})
		return
	}

	// var req SetReactionRequest
	// if err := c.ShouldBindJSON(&req); err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求体解析失败: %v", err)})
	// 	return
	// }

	reaction := string(tools.Match(c.GetRawData()).Result())

	err = setReactionAlt(tid, reaction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("设置reaction失败: %v", err)})
		return
	}

	message := "Reaction操作成功"
	if reaction == "" || reaction == "none" {
		message = "Reaction已移除"
	} else {
		message = fmt.Sprintf("Reaction已设置为: %s", reaction)
	}

	c.JSON(http.StatusOK, gin.H{"message": message})
}
