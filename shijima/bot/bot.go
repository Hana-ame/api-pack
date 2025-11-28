package bot

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

func Handler(c *gin.Context) {
	bot := c.Param("bot")
	tid := tools.Atoi(c.Param("tid"), 0)
	query := c.Query("q")

	if !strings.HasPrefix(bot, "@") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "bot name : " + bot,
		})
	}
	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "tid :" + c.Param("tid"),
		})
	}

	switch c.Request.Method {
	// 处理POST. 其实是预留接口处理来自别的岛实例的. 暂时不用了 250530
	case http.MethodPost:
		// body, err := io.ReadAll(c.Request.Body)
		// defer c.Request.Body.Close()
		// if err != nil {
		// 	c.JSON(http.StatusBadRequest, gin.H{
		// 		"error": err.Error(),
		// 	})
		// 	return
		// }
		body := []byte(c.GetString("thread"))
		err := Request(int64(tid), bot, query, body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"@type": "error",
				"error": err.Error(),
			})
			return
		}
		c.AbortWithStatus(http.StatusOK)

	case http.MethodGet:
		body, status, err := Response(int64(tid), bot, query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"@type":  "error",
				"status": status,
				"error":  err.Error(),
			})
			return
		}
		switch status {
		case "done":
			c.Data(http.StatusOK, "application/json", body)
		case "pending":
			c.JSON(http.StatusOK, gin.H{"@type": "status", "status": status})
		default:
			c.Data(http.StatusOK, "plain/text", []byte(status))
		}

	default:
		c.JSON(http.StatusForbidden, gin.H{"@type": "error", "error": "method not allowed" + c.Request.Method})
	}

	return
}

func Request(tid int64, bot string, query string, body []byte) error {
	switch bot {
	case "@board", "@rd", "@random":
		return nil
	case "@deepseek":
		return deepseekRequest(tid, bot, query, body)
	case "@reaction":
		return reactionRequest(tid, bot, query, body)
	default:
		return nil
	}
}

func Response(tid int64, bot string, query string) ([]byte, string, error) {
	switch bot {
	case "@board":
		return boardResponse(query)
	case "@rd", "@random":
		return randomResponse(tid, query)
	case "@reaction":
		return []byte{}, "done", nil
	default:
		record, err := GetRecord(tid, bot, query)
		if err != nil {
			return []byte{}, "failed", err
		}
		if record == nil {
			return []byte{}, "failed", nil // 没有记录，返回 pending 状态
		}
		return []byte(record.Response), record.Status, nil
	}
}

// ===================================

func deepseekRequest(tid int64, bot string, query string, body []byte) error {
	// InsertOrUpdate(tid, bot, query, "pending", "{}")
	// InsertOrUpdate(tid, bot, query, "done", respJSON)
	return nil
}

func reactionRequest(tid int64, bot string, query string, body []byte) error {
	om := orderedmap.New()
	if err := json.Unmarshal(body, &om); err != nil {
		return err
	}

	r := int(om.GetOrDefault("r", float64(0)).(float64))
	if r == 0 {
		r = int(om.GetOrDefault("no", float64(0)).(float64))
		if r == 0 {
			return nil
		}
	}

	sqlquery := `
		INSERT INTO reactions_alt (tid, reaction, count)
		VALUES (?, ?, 1)
		ON DUPLICATE KEY UPDATE count = count + 1, timestamp = CURRENT_TIMESTAMP;
	`
	_, err := DB().Exec(sqlquery, r, []byte(query))

	return err
}

func boardResponse(query string) ([]byte, string, error) {
	bid, name, err := tools.SeprateString(" ", query)
	if err != nil {
		id := tools.Atoi(query, 0)
		if id == 0 {
			return nil, "failed", fmt.Errorf("not valid : %s", query)
		}
		body, err := json.Marshal(map[string]string{
			"@type": "board",
			"bid":   strconv.Itoa(id),
		})
		if err != nil {
			return nil, "failed", err
		}
		return body, "done", nil
	} else {
		id := tools.Atoi(bid, 0)
		if id == 0 {
			return nil, "failed", fmt.Errorf("not valid : %s", bid)
		}
		body, err := json.Marshal(map[string]string{
			"@type": "board",
			"bid":   strconv.Itoa(id),
			"name":  name,
		})
		if err != nil {
			return nil, "failed", err
		}
		return body, "done", nil
	}
}

func randomResponse(tid int64, query string) ([]byte, string, error) {
	rs := rand.New(rand.NewSource(tid))

	arr := strings.Split(query, "d")
	if len(arr) != 2 {
		bodyJSON, err := json.Marshal(gin.H{
			"@type": "error",
			"error": "query not valid",
			"query": query,
			"tid":   tid,
		})
		return bodyJSON, "failed", err
	}

	t := tools.Atoi(arr[0], 0)
	d := tools.Atoi(arr[1], 1)

	if t > 100 {
		bodyJSON, err := json.Marshal(gin.H{
			"@type": "error",
			"error": "query not valid",
			"query": query,
			"tid":   tid,
		})
		return bodyJSON, "failed", err
	}

	sum := 0
	result := ""
	for i := 0; i < t; i++ {
		r := rs.Intn(d) + 1
		sum += r
		// result += " + " + strconv.Itoa(r)
	}
	// if t > 1 {
	// 	result += " = " + strconv.Itoa(sum)
	// } else {
	result = query + " = " + strconv.Itoa(sum)
	// }
	bodyJSON, err := json.Marshal(gin.H{
		"@type": "text",
		"text":  result,
		"tid":   tid,
	})

	return bodyJSON, "done", err
}
