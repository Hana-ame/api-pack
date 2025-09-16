package shijima

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"

	// "os" // 如果不再需要 os.Remove(dbPath) 可以删除

	tools "github.com/Hana-ame/api-pack/Tools"
	"github.com/Hana-ame/api-pack/Tools/sqlite"
	"github.com/gin-gonic/gin"
)

// Record 结构体现在应该从 sqlite 包中导入
// type Record struct {
// 	Key   int64  `json:"key"`
// 	Value string `json:"value"`
// }

// var coverDB *sqlite.VSQLiteDB // 全局数据库实例，类型改为 VSQLiteDB
var coverDB, _ = sqlite.NewVSQLiteDB("cover.db", "cover")         // 注意这里是 NewVSQLiteDB
var bilicoverDB, _ = sqlite.NewVSQLiteDB("cover.db", "bilicover") // 注意这里是 NewVSQLiteDB

// func main() {
// 	// 初始化 SQLite 数据库
// 	dbPath := "my_urls.db" // 数据库文件路径
// 	tableName := "urls"    // 表名
// 	var err error
// 	kvDB, err = sqlite.NewVSQLiteDB(dbPath, tableName) // 注意这里是 NewVSQLiteDB
// 	if err != nil {
// 		log.Fatalf("初始化数据库失败: %v", err)
// 	}
// 	defer func() {
// 		if err := kvDB.Close(); err != nil {
// 			log.Printf("关闭数据库连接失败: %v", err)
// 		} else {
// 			log.Println("数据库连接已关闭。")
// 		}
// 	}()

// 	// 初始化 Gin 路由器
// 	r := gin.Default()

// 	// 注册路由
// 	r.GET("/random-record", getRandomRecordHandler)
// 	r.POST("/add-url", addURLHandler)

// 	// 启动服务器
// 	port := ":8080"
// 	log.Printf("Gin 服务器正在端口 %s 上运行...", port)
// 	if err := r.Run(port); err != nil {
// 		log.Fatalf("Gin 服务器启动失败: %v", err)
// 	}
// }

func getRandomHandler(c *gin.Context) {
	table := tools.Or(c.Query("table"), "cover")
	var key int64
	var value string
	querySQL := fmt.Sprintf(`SELECT key, value FROM %s ORDER BY RANDOM() LIMIT 1;`, table)
	err := coverDB.DB().QueryRow(querySQL).Scan(&key, &value)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error(), "message": "table not found"})
		return
	}

	c.Redirect(http.StatusFound, value)
}

// addURLHandler 接收一个 URL 并保存到表中
func addRandomHandler(c *gin.Context) {
	table := tools.Or(c.Query("table"), "cover")
	if table == "cover" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "换个表名（hint: ?table={tableName}）"})
		return
	}

	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			key INTEGER PRIMARY KEY AUTOINCREMENT,
			value TEXT
		);
	`, table)

	_, err := coverDB.DB().Exec(createTableSQL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "table": table})
	}

	// 从请求体中读取 plain/text 数据
	urlBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取请求体"})
		return
	}
	url, err := url.Parse(string(urlBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "url": string(urlBytes)})
		return
	}

	// 保存 URL 到数据库
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (value) VALUES (?);
	`, table)
	result, err := coverDB.DB().Exec(insertSQL, url.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "url": url.String()})
		return
	}
	lastID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "URL 保存成功",
		"table":   table,
		"key":     lastID,
		"url":     url.String(),
	})
}

// ================================================================================
// ================================================================================
// =========below is cover=========================================================
// =========before is random=======================================================
// ================================================================================
// ================================================================================

// getRandomRecordHandler 随机读取一个记录并返回
func getRandomRecordHandler(c *gin.Context) {
	record, err := coverDB.GetRandomRecord() // 调用 sqlite 包中的方法
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "表中没有记录"})
			return
		}
		log.Printf("获取随机记录失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取随机记录"})
		return
	}

	c.Redirect(http.StatusFound, record.Value)
}

// getRandomRecordHandler 随机读取一个记录并返回
func getRandomRecordHandlerBili(c *gin.Context) {
	record, err := bilicoverDB.GetRandomRecord() // 调用 sqlite 包中的方法
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "表中没有记录"})
			return
		}
		log.Printf("获取随机记录失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取随机记录"})
		return
	}

	u, err := url.Parse(record.Value)
	if tools.AbortWithError(c, 500, err) {
		return
	}

	// u.Query().Add("proxy_host", u.Host)
	u.Scheme, u.Host, u.RawQuery = "https", "proxy.moonchan.xyz", "proxy_host="+u.Host

	c.Redirect(http.StatusFound, u.String())
}

// addURLHandler 接收一个 URL 并保存到表中
func addURLHandler(c *gin.Context) {
	// 从请求体中读取 plain/text 数据
	urlBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取请求体"})
		return
	}
	url := string(urlBytes)

	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL 不能为空"})
		return
	}

	// 保存 URL 到数据库
	newKey, err := coverDB.AddValue(url)
	if err != nil {
		log.Printf("保存 URL 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存 URL 失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "URL 保存成功",
		"key":     newKey,
		"url":     url,
	})
}
