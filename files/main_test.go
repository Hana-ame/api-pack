package files

import (
	db "api-pack/Tools/db_filehash"

	_ "github.com/mattn/go-sqlite3"

	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/ws", wsHandler)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.Run(":8080")
}

func TestXxx(t *testing.T) {
	main()
}

func TestDBCreate(t *testing.T) {
	db.CreatePathHash("./foo.db", "foo")
	db.CreatePathHash("./files.go", "files")
	db.CreatePathHash("./ws.go", "ws")
	path, err := db.ReadPathByHash("test")
	fmt.Println(path, err)
}

func TestInit(t *testing.T) {

	r := gin.Default()
	r.GET("/file/:sha1sum", LocalFile)
	err := r.Run("127.24.9.18:8080")
	fmt.Println(err)

}
