// 需要在psql中创建表 id - mime_type
// 需要指定 CONN_STR
// 可选 UPLOAD_PATH
// ~~超过1m就会520~~ 是nginx设置，但是nginx设置好了cloudflare对于大文件上传也是恨得很

package handler

import (
	"database/sql"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq" // 导入 PostgreSQL 驱动
)

// setupDB 初始化数据库连接并返回一个事务
func SetupDB() (db *sql.DB, tx *sql.Tx, err error) {
	// connStr 是数据库连接字符串，可以从环境变量中获取
	var connStr = os.Getenv("CONN_STR")
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return
	}

	// 开始一个事务
	tx, err = db.Begin()
	if err != nil {
		return
	}

	return
}

func ReadFileMIMEType(tx *sql.Tx, id int64) (mimeType string, err error) {
	query := `
	SELECT mime_type
	FROM files
	WHERE id = $1
	;`
	row := tx.QueryRow(query, id)
	// 扫描数据库返回的结果并赋值给 Account 结构体
	if err := row.Scan(
		&mimeType,
	); err != nil {
		return mimeType, fmt.Errorf("could not retrieve mime_type: %v", err)
	}

	return
}

func CreateFileMIMEType(tx *sql.Tx, id int64, mimeType string) (err error) {
	query := `
	INSERT INTO files (id, mime_type)
	VALUES ($1, $2)
	`
	// 执行插入操作
	if _, err := tx.Exec(query, id, mimeType); err != nil {
		return err
	}

	return
}

func ReadFiles(tx *sql.Tx, mimeTypes []string) (idArr []int64, err error) {
	var args []any

	query := `
	SELECT id
	FROM files
	ORDER BY id DESC
	LIMIT 10
	;`
	if len(mimeTypes) > 0 {
		offset := 1
		placeholders := make([]string, len(mimeTypes))
		args = make([]any, len(mimeTypes))
		for i, mt := range mimeTypes {
			placeholders[i] = fmt.Sprintf("$%d", i+offset)
			args[i] = mt
		}

		query = fmt.Sprintf(`
            SELECT id
            FROM files
            WHERE mime_type IN (%s)
            ORDER BY id DESC
            LIMIT 10;`,
			strings.Join(placeholders, ","),
		)
	}

	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve files: %v", err)
	}
	defer rows.Close()

	idArr = make([]int64, 0, 10)

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("could not scan row: %v", err)
		}
		idArr = append(idArr, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return
}

func ReadFilesBefore(tx *sql.Tx, before int64, mimeTypes []string) (idArr []int64, err error) {
	if before == 0 {
		return ReadFiles(tx, mimeTypes)
	}

	args := make([]any, len(mimeTypes)+1)
	args[0] = before

	query := `
	SELECT id
	FROM files
	WHERE id < $1
	ORDER BY id DESC
	LIMIT 10
	;`
	if len(mimeTypes) > 0 {
		offset := 2
		placeholders := make([]string, len(mimeTypes))
		for i, mt := range mimeTypes {
			placeholders[i] = fmt.Sprintf("$%d", i+offset)
			args[i+1] = mt
		}

		query = fmt.Sprintf(`
				SELECT id
				FROM files
				WHERE id < $1
				AND mime_type IN (%s)
				ORDER BY id DESC
				LIMIT 10;`,
			strings.Join(placeholders, ","),
		)
	}

	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve files: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("could not scan row: %v", err)
		}
		idArr = append(idArr, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return
}

// PUT
func UploadFilePsql(c *gin.Context) {
	id := tools.NewTimeStamp()
	idStr := strconv.Itoa(int(id))
	idStrArr := []string{idStr[0:3], idStr[3:6], idStr[6:]}
	// fn := idStrArr[len(idStrArr)-1]
	// MIME type 通过 c.ContentType() 决定
	mimeType := tools.Or(c.ContentType(), "application/octet-stream")
	extensions, _ := mime.ExtensionsByType(mimeType)
	extension := tools.Or(tools.NewSlice(extensions...).FirstUnequal(""), ".bin")

	if err := os.MkdirAll(filepath.Join(os.Getenv("UPLOAD_PATH"), idStrArr[0], idStrArr[1]), 0755); err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	filePath := strings.Join(idStrArr, "/") + extension

	fileWriter, err := os.Create(filepath.Join(os.Getenv("UPLOAD_PATH"), filePath))
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer fileWriter.Close()

	n, err := io.Copy(fileWriter, c.Request.Body)
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if n != c.Request.ContentLength {
		c.Header("X-Error", fmt.Sprintf("n = %d, expected %d", n, c.Request.ContentLength))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	db, tx, err := SetupDB()
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer db.Close()
	defer tx.Rollback()

	err = CreateFileMIMEType(tx, id, mimeType)
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, map[string]any{
		"id": idStr,
	})
}

// :id :filename
func DownloadFilePsql(c *gin.Context) {

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	db, tx, err := SetupDB()
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer db.Close()
	defer tx.Rollback()

	// MIME 从数据库读取
	mimeType, err := ReadFileMIMEType(tx, int64(id))
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	idStrArr := []string{idStr[0:3], idStr[3:6], idStr[6:]}
	// 通过 MIME 还原扩展名，找文件。 这一步是为了能够在文件夹当中预览文件而不是只能看一个白板
	extensions, _ := mime.ExtensionsByType(mimeType)
	extension := tools.Or(tools.NewSlice(extensions...).FirstUnequal(""), ".bin")
	filePath := strings.Join(idStrArr, "/") + extension

	fileReader, err := os.Open(filepath.Join(os.Getenv("UPLOAD_PATH"), filePath))
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer fileReader.Close()

	// 获取文件信息
	fileInfo, err := os.Stat(filepath.Join(os.Getenv("UPLOAD_PATH"), filePath))
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	// MIME 也会在游览器返回。
	c.DataFromReader(http.StatusOK, fileInfo.Size(), mimeType, fileReader, map[string]string{
		"Content-Disposition": "inline",
		"Cache-Control":       "public, max-age=31536000, immutable",
		"Last-Modified":       "Tue, 27 May 2025 00:00:00 GMT",
		"Expires":             time.Now().Add(365 * 24 * time.Hour).Format(http.TimeFormat),
	})

}

// /list?before={before}
func ListFilesPsql(c *gin.Context) {
	before := tools.Atoi(c.Query("before"), 0)

	db, tx, err := SetupDB()
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer db.Close()
	defer tx.Rollback()

	// MIME 从数据库读取
	mimeTypes := []string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml"}

	idArr, err := ReadFilesBefore(tx, int64(before), mimeTypes)
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, idArr)
}
