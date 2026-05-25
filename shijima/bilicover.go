package shijima

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"

	"github.com/Hana-ame/api-pack/utils/sqlite"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
)

var bilicoverDB, _ = sqlite.NewVSQLiteDB("cover.db", "bilicover") // 注意这里是 NewVSQLiteDB

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
