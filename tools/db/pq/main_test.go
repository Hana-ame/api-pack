package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	_ "github.com/joho/godotenv/autoload"

	_ "github.com/lib/pq"
)

func TestConnection(t *testing.T) {
	connStr := os.Getenv("CONN_STR")
	log.Println(connStr)
	for i := 0; i < 2; i++ {
		// 连接到 PostgreSQL 数据库
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			log.Fatal("Error connecting to the database: ", err)
		}
		defer db.Close()

		// 测试连接
		err = db.Ping()
		if err != nil {
			log.Fatal("Error pinging the database: ", err)
		}
		fmt.Println("Successfully connected to the database!")

		fmt.Printf("%v", db)
	}
}

func TestMessage(t *testing.T) {
	o := orderedmap.New()
	o.Set("key", "value")
	payload, _ := o.MarshalJSON()

	createMessage := func(tx *sql.Tx) error {
		id := tools.GetTimestamp()
		result, err := tx.Exec(`INSERT INTO messages (id, receiver, sender, payload) VALUES ($1, $2, $3, $4);`, id, "reciver", "sender", payload)
		fmt.Println(err)
		r, e := result.RowsAffected()
		fmt.Println(r, e)
		tx.Commit()
		return err
	}

	// readMessages 是一个函数，用于从数据库中读取消息。
	readMessages := func(tx *sql.Tx) error {
		rows, err := tx.Query("SELECT id, receiver, sender, payload FROM messages")
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var id int
			var receiver string
			var sender string
			var payload []byte
			if err := rows.Scan(&id, &receiver, &sender, &payload); err != nil {
				return err
			}

			fmt.Println(id, receiver, sender, payload)
		}

		// 检查遍历行时是否出错
		if err := rows.Err(); err != nil {
			return err
		}

		tx.Commit()

		return nil
	}
	var err error
	err = Exec(createMessage)
	fmt.Println(err)
	err = Exec(readMessages)
	fmt.Println(err)

}
