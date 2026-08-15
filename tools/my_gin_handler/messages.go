package handler

import (
	"database/sql"
	"net/http"

	db "github.com/Hana-ame/api-pack/tools/db/pq"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

type message struct {
	ID       string `json:"id"`
	Receiver string `json:"receiver"`
	Sender   string `json:"sender"`
	Payload  []byte `json:"payload"`
}

// :receiver ?receiver
// :sender ?sender X-Forwarded-For
func SendMsg(c *gin.Context) {
	receiver := tools.NewSlice(c.GetString("receiver"), c.Param("receiver"), c.Query("receiver")).FirstUnequal("")
	sender := tools.NewSlice(c.GetString("sender"), c.Param("sender"), c.Query("sender"), c.GetHeader("X-Forwarded-For")).FirstUnequal("")

	id := tools.NewTimeStamp()

	blob, err := c.GetRawData()
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusBadRequest)
	}

	// o := orderedmap.New()
	// if err := json.Unmarshal(blob, &o); err != nil {
	// 	c.Header("X-Error", err.Error())
	// 	c.AbortWithStatus(http.StatusBadRequest)
	// }

	if err := db.Exec(func(tx *sql.Tx) error {

		query := `INSERT INTO messages (id, receiver, sender, payload) VALUES ($1, $2, $3, $4);`
		if _, err := tx.Exec(query, id, receiver, sender, blob); err != nil {
			return err
		}
		return tx.Commit()
	}); err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.AbortWithStatus(http.StatusNoContent)
}

// :receiver
// ?after
// ?limit
func ReceiveMsg(c *gin.Context) {
	receiver := tools.NewSlice(c.GetString("receiver"), c.Param("receiver"), c.Query("receiver")).FirstUnequal("")
	afterString := tools.NewSlice(c.GetString("after"), c.Param("after"), c.Query("after")).FirstUnequal("")
	after := tools.Atoi(afterString, 0)
	limitString := tools.NewSlice(c.GetString("limit"), c.Param("limit"), c.Query("limit")).FirstUnequal("")
	limit := tools.Atoi(limitString, 10)
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	messages := make([]message, 0, limit) // Initialize with capacity, not length

	if err := db.Exec(func(tx *sql.Tx) error {
		rows, err := tx.Query("SELECT id, receiver, sender, payload FROM messages WHERE receiver = $1 AND id > $2 ORDER BY id ASC LIMIT $3", receiver, after, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var msg message
			// var blob []byte
			if err := rows.Scan(&msg.ID, &msg.Receiver, &msg.Sender, &msg.Payload); err != nil {
				return err
			}
			// if err := json.Unmarshal(blob, &msg.Payload); err != nil {
			// 	return err
			// }

			messages = append(messages, msg)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		return tx.Commit()
	}); err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSONP(http.StatusOK, messages)
}
