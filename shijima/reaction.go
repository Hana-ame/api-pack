package shijima

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

// getReactions 根据 tid 查询反馈，并返回统计结果及“我的”反馈
//
// 注意：为了实现 `my_reaction`，需要知道“我的” IP。
// 鉴于函数签名 `func getReactions(tid int)`，我在此处使用了硬编码的 `myIPPlaceholder`。
// 在实际应用中，`myIP` 应该作为参数传入（例如：`func getReactions(tid int, myIP string)`）
// 或者从上下文/会话中获取。
func getReactions(tid int, myIP string) (*orderedmap.OrderedMap, string, error) {
	rows, err := db.Query(
		`SELECT ip, reaction
		FROM reactions
		WHERE tid = ?`,
		tid,
	)
	if err != nil {
		return nil, "", fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	// 使用 map 来聚合 reaction 的计数
	resultMap := orderedmap.New()
	var myReaction string // 存储当前用户的 reaction

	for rows.Next() {
		var ip, reaction string
		if err := rows.Scan(&ip, &reaction); err != nil {
			return nil, myReaction, fmt.Errorf("扫描行失败: %w", err)
		}

		// 累加 reaction 计数
		resultMap.Set(reaction, resultMap.GetOrDefault(reaction, 0).(int)+1)

		// 如果当前行的 IP 是“我的” IP，则记录下这个 reaction
		// 如果一个用户对同一个 tid 有多个 reaction，这里会记录最后一个
		// 如果需要记录第一个或所有，需要修改逻辑
		if ip == myIP {
			myReaction = reaction
		}
	}

	// 检查迭代过程中的错误
	if err = rows.Err(); err != nil {
		return nil, myReaction, fmt.Errorf("遍历行错误: %w", err)
	}

	return resultMap, myReaction, nil
}

// setReaction 设置或更新用户的反馈
// tid: 帖子ID
// myIP: 用户IP地址
// reaction: 要设置的反馈（如 "👍", "❤️"）。
//
//	如果 reaction 为空字符串或特定“取消”值（例如 "none" 或 "cancel"），则表示取消/删除反馈。
func setReaction(tid int, myIP string, reaction string) error {
	// 启动事务，确保操作的原子性
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("无法开始事务: %w", err)
	}
	defer tx.Rollback() //：如果函数在Commit之前返回错误，会自动回滚
	defer func() {
		if r := recover(); r != nil { // 捕获 panic
			tx.Rollback()
			panic(r) // 重新抛出 panic
		} else if err != nil { // 如果存在错误 (err != nil)，则回滚
			tx.Rollback()
		}
	}()

	// 1. 查询用户是否已对该 tid 做出反应
	var oldReaction string
	// 使用 ForUpdate 锁住行，防止并发更新问题（某些数据库可能不支持，如 SQLite）
	// For SQLite, a simple SELECT is usually fine as it uses table-level locks.
	// For MySQL/PostgreSQL, consider "FOR UPDATE"
	err = tx.QueryRow(`SELECT reaction FROM reactions WHERE tid = ? AND ip = ?`, tid, myIP).Scan(&oldReaction)

	oldReactionFound := true
	if err == sql.ErrNoRows {
		oldReactionFound = false
	} else if err != nil {
		return fmt.Errorf("查询现有reaction失败: %w", err)
	}

	// 判断新的 reaction 是否意味着取消/删除
	// 定义一个取消指令，这里以空字符串或 "none" 为例
	isCancelling := (reaction == "" || reaction == "none")

	if oldReactionFound {
		if isCancelling {
			// 用户有旧 reaction，现在想取消它
			_, err = tx.Exec(`DELETE FROM reactions WHERE tid = ? AND ip = ?`, tid, myIP)
			if err != nil {
				return fmt.Errorf("删除reaction失败: %w", err)
			}
		} else if oldReaction != reaction {
			// 用户有旧 reaction，现在想更新为新的不同 reaction
			_, err = tx.Exec(`UPDATE reactions SET reaction = ? WHERE tid = ? AND ip = ?`, reaction, tid, myIP)
			if err != nil {
				return fmt.Errorf("更新reaction失败: %w", err)
			}
		}
		// else if oldReaction == reaction, do nothing, it's already set.
	} else { // 没有找到旧 reaction
		if !isCancelling {
			// 用户没有 reaction，现在想添加一个新的 reaction
			_, err = tx.Exec(`INSERT INTO reactions (tid, ip, reaction) VALUES (?, ?, ?)`, tid, myIP, reaction)
			if err != nil {
				return fmt.Errorf("插入reaction失败: %w", err)
			}
		}
		// else if isCancelling and no old reaction, do nothing (cannot cancel something that doesn't exist)
	}

	// 如果所有操作都成功，提交事务
	return tx.Commit()
}

// GetReactionsHandler 处理获取帖子 reactions 的 HTTP 请求
// GET /reactions/:tid
func GetReactionsHandler(c *gin.Context) {
	tidStr := c.Param("tid")
	tid, err := strconv.Atoi(tidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的tid参数"})
		return
	}

	myIP := c.GetHeader("X-Forwarded-For") // 获取客户端IP

	reactionCountsMap, myReaction, err := getReactions(tid, myIP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取reactions失败: %v", err)})
		return
	}

	response := gin.H{
		"reactions":   reactionCountsMap,
		"my_reaction": myReaction,
	}

	c.JSON(http.StatusOK, response)
}

// SetReactionRequest 用于解析设置 reaction 请求的 JSON 体
// type SetReactionRequest struct {
// 	Reaction string `json:"reaction"` // 如果为空字符串，则表示取消 reaction
// }

// SetReactionHandler 处理设置/更新/取消用户 reaction 的 HTTP 请求
// POST /reactions/:tid
func SetReactionHandler(c *gin.Context) {
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

	myIP := c.GetHeader("X-Forwarded-For") // 获取客户端IP
	// 注意：同样适用于生产环境的IP获取考虑

	err = setReaction(tid, myIP, reaction)
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

// 定义一个结构体来封装每个帖子 reactions 的结果，用于并发处理
type ReactionResult struct {
	TID        int
	Counts     *orderedmap.OrderedMap
	MyReaction string
	Error      error
}

// getReactionsBatch 负责并发获取多个 tid 的 reactions
func getReactionsBatch(tids []int, myIP string) (map[int]*orderedmap.OrderedMap, map[int]string, error) {
	// 使用通道来收集并发操作的结果
	resultsChan := make(chan ReactionResult, len(tids))
	var wg sync.WaitGroup // 使用 WaitGroup 等待所有 Goroutine 完成

	for _, tid := range tids {
		wg.Add(1) // 增加计数器
		go func(currentTID int) {
			defer wg.Done() // Goroutine 完成时减少计数器

			counts, myReaction, err := getReactions(currentTID, myIP) // 调用获取单个 reactions 的函数
			resultsChan <- ReactionResult{                            // 将结果发送到通道
				TID:        currentTID,
				Counts:     counts,
				MyReaction: myReaction,
				Error:      err,
			}
		}(tid) // 将 tid 传递给 Goroutine
	}

	wg.Wait()          // 等待所有 Goroutine 完成
	close(resultsChan) // 关闭通道，表示没有更多数据会发送

	// 收集所有结果
	allReactionCounts := make(map[int]*orderedmap.OrderedMap)
	allMyReactions := make(map[int]string)
	var firstError error // 存储第一个遇到的错误

	for res := range resultsChan {
		if res.Error != nil {
			// 如果有任何一个 tid 获取失败，我们选择返回整个批处理的错误。
			// 另一种策略是收集所有错误，或者跳过失败的 tid。这里采取简单策略。
			if firstError == nil {
				firstError = fmt.Errorf("failed to get reactions for tid %d: %w", res.TID, res.Error)
			}
			// continue // 如果你想跳过失败的，而不是整个批处理失败，可以使用 continue
		} else {
			allReactionCounts[res.TID] = res.Counts
			allMyReactions[res.TID] = res.MyReaction
		}
	}

	return allReactionCounts, allMyReactions, firstError
}

// GetReactionsBatchHandler 处理获取多个帖子 reactions 的 HTTP 请求
// GET /reactions_batch?id=1&id=2&id=3 或 /reactions_batch?id=1,2,3
func GetReactionsBatchHandler(c *gin.Context) {
	// 获取所有名为 "id" 的查询参数，Gin 会将它们作为字符串切片返回
	// 例如：?id=1&id=2 -> ["1", "2"]
	// 或者：?id=1,2,3 -> ["1,2,3"] （如果前端只传一个带逗号分隔的参数）
	// 我们这里处理两种情况，但更推荐前端发送 ?id=1&id=2&id=3 这种形式
	tidStrings := c.QueryArray("id[]")

	if len(tidStrings) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要一个id参数"})
		return
	}

	tids := make([]int, 0, len(tidStrings))
	for _, tidStr := range tidStrings {
		// 检查 tidStr 是否包含逗号，以处理 id=1,2,3 的情况
		if containsComma(tidStr) {
			parts := splitByComma(tidStr)
			for _, part := range parts {
				tid, err := strconv.Atoi(part)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的id参数 '%s'", part)})
					return
				}
				tids = append(tids, tid)
			}
		} else {
			tid, err := strconv.Atoi(tidStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的id参数 '%s'", tidStr)})
				return
			}
			tids = append(tids, tid)
		}
	}

	// 确保 ID 列表不为空
	if len(tids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未提供有效的id参数"})
		return
	}

	// 确保 ID 列表唯一，避免重复查询和前端重复解析
	uniqueTIDsMap := make(map[int]bool)
	uniqueTIDs := []int{}
	for _, tid := range tids {
		if _, ok := uniqueTIDsMap[tid]; !ok {
			uniqueTIDsMap[tid] = true
			uniqueTIDs = append(uniqueTIDs, tid)
		}
	}

	myIP := c.GetHeader("X-Forwarded-For") // 获取客户端IP

	reactionCountsMapBatch, myReactionBatch, err := getReactionsBatch(uniqueTIDs, myIP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取批量reactions失败: %v", err)})
		return
	}

	// 构建响应结构：{ "tid1": { "reactions": {...}, "my_reaction": "..." }, "tid2": {...} }
	responseMap := make(map[string]gin.H)
	for tid, counts := range reactionCountsMapBatch {
		responseMap[strconv.Itoa(tid)] = gin.H{ // 将 int tid 转换为 string key
			"reactions":   counts,
			"my_reaction": myReactionBatch[tid],
		}
	}

	c.JSON(http.StatusOK, responseMap)
}

// 辅助函数：检查字符串是否包含逗号
func containsComma(s string) bool {
	return len(s) > 0 && s[0] != ',' && s[len(s)-1] != ',' && strings.Contains(s, ",") // 避免空字符串或首尾是逗号
}

// 辅助函数：按逗号分割字符串
func splitByComma(s string) []string {
	return strings.Split(s, ",")
}
