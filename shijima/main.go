// wait for test

package shijima

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	_ "github.com/Hana-ame/api-pack/Tools"
	tools "github.com/Hana-ame/api-pack/Tools"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	handler "github.com/Hana-ame/api-pack/Tools/my_gin_handler"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/Hana-ame/api-pack/Tools/randomreader"
	"github.com/Hana-ame/api-pack/Tools/sqlite"
	"github.com/Hana-ame/api-pack/shijima/bot"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-multierror"
	"github.com/nfnt/resize"
)

var kv, _ = sqlite.NewKVSQLiteDB("kv.db", "query_url")

func uploadAndCache(image image.Image, path string) error {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, image, &jpeg.Options{Quality: 80}) // 改为写入 buf
	if err != nil {
		return fmt.Errorf("编码图片失败: %w", err)
	}

	resp, err := myfetch.Fetch(http.MethodPut, "https://upload.moonchan.xyz/api/upload", http.Header{
		"Content-Type":   []string{"image/jpeg"},
		"Content-Length": []string{fmt.Sprintf("%d", buf.Len())},
	}, &buf)
	if err != nil {
		return fmt.Errorf("上传图片失败: %w", err)
	}
	defer resp.Body.Close()

	o, err := tools.ReaderToJSON(resp.Body)
	if err != nil {
		return fmt.Errorf("error on decode: %w", err)
	}

	// 缓存到 KV 数据库
	if err := kv.AddOrUpdate(path, o.GetOrDefault("id", "").(string)); err != nil {
		return fmt.Errorf("缓存图片失败: %w", err)
	}

	return nil
}

var db, _ = func() (*sql.DB, error) {
	dsn := os.Getenv("MARIADB")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// 连接池参数
	db.SetMaxOpenConns(100)                 // 最大活跃连接数
	db.SetMaxIdleConns(30)                  // 最大空闲连接数
	db.SetConnMaxLifetime(5 * time.Minute)  // 连接最长存活时间
	db.SetConnMaxIdleTime(30 * time.Minute) // 空闲连接最长保留时间

	return db, nil
}()

const pageSize = 30

// 表结构对应的结构体（根据图片中的列定义）
type Thread struct {
	T    string    `db:"t" json:"t,omitempty"`  // title (无标题)
	N    string    `db:"n" json:"n,omitempty"`  // name (无名氏)
	Ts   string    `db:"ts" json:"ts"`          // timestamp
	ID   string    `db:"id" json:"id"`          // user identity
	No   uint      `db:"no" json:"no"`          // number
	P    string    `db:"p"  json:"p,omitempty"` // picture src
	Txt  string    `db:"txt" json:"txt"`        // content
	R    uint      `db:"r" json:"r,omitempty"`  // reply to
	Del  int8      `db:"del" json:"-"`          // is deleted?
	C    string    `db:"c" json:"-"`            // country
	IP   string    `db:"ip" json:"-"`           // ip address
	Num  int       `json:"num,omitempty"`       // from board
	List []*Thread `json:"list,omitempty"`      // replies
}

// 表结构对应的结构体（根据图片中的列定义）
type Board struct {
	TID      uint `db:"tid" json:"tid,omitempty"`
	BID      uint `db:"bid" json:"bid,omitempty"`
	ReplyNum uint `db:"num"`
}

// 就是找到单条thread
func getThreadByNo(no int) (*Thread, error) {
	var thread Thread
	thread.No = uint(no)
	err := db.QueryRow(
		`SELECT t,n,ts,id,p,txt,r
		FROM thread
		WHERE no = ? AND del >= 0;`,
		no,
	).Scan(
		&thread.T, &thread.N, &thread.Ts, &thread.ID, &thread.P, &thread.Txt, &thread.R,
	)
	if err != nil {
		if err == sql.ErrNoRows || thread.Del < 0 {
			return nil, fmt.Errorf("主题不存在")
		}
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	if err == nil && thread.R != 0 {
		err := db.QueryRow(
			`SELECT del
			FROM thread
			WHERE no = ? AND del >= 0;`,
			thread.R,
		).Scan(
			&thread.Del,
		)
		if err != nil || thread.Del < 0 {
			if err == sql.ErrNoRows || thread.Del < 0 {
				return nil, fmt.Errorf("主题不存在")
			}
			return nil, fmt.Errorf("数据库查询失败: %w", err)
		}
	}

	if err == nil && thread.R == 0 {
		err := db.QueryRow(
			`SELECT replynum
			FROM board
			WHERE tid = ?`,
			thread.No,
		).Scan(
			&thread.Num,
		)
		if err != nil {
			if err == sql.ErrNoRows || thread.Del < 0 {
				return nil, fmt.Errorf("主题不存在")
			}
			return nil, fmt.Errorf("数据库查询失败: %w", err)
		}
	}

	return &thread, nil
}

// 获得回复no的最新的5条作为replies
func getReplies(no, pn int) ([]*Thread, error) {
	replies := make([]*Thread, 0, pageSize)

	offset := pn * (pageSize)

	// 执行分页查询
	rows, err := db.Query(
		`SELECT t, n, ts, id, no, p, txt 
         FROM thread 
         WHERE r = ? AND del >= 0 
         ORDER BY no ASC 
         LIMIT ? OFFSET ?`,
		no, pageSize, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	// 遍历结果集
	for rows.Next() {
		var t Thread
		if err := rows.Scan(
			&t.T, &t.N, &t.Ts, &t.ID, &t.No, &t.P, &t.Txt,
		); err != nil {
			return nil, fmt.Errorf("数据解析失败: %w", err)
		}
		replies = append(replies, &t)
	}

	// 检查遍历错误
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("结果集处理错误: %w", err)
	}

	return replies, nil
}

// 获得最新的5条作为replies
func getRepliesPreview(no int) ([]*Thread, error) {
	replies := make([]*Thread, 0, 5)

	// 使用 Query 获取多行结果（网页5][网页7）
	rows, err := db.Query(
		`SELECT t, n, ts, id, no, p, txt 
        FROM thread 
        WHERE r = ? AND del >= 0 
        ORDER BY no DESC 
        LIMIT 5`, // 网页6 分页语法参考
		no,
	)
	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}
	defer rows.Close() // 确保关闭连接（网页7强调）

	// 遍历结果集（网页5][网页7）
	for rows.Next() {
		var t Thread
		if err := rows.Scan(
			&t.T, &t.N, &t.Ts, &t.ID, &t.No, &t.P, &t.Txt,
		); err != nil {
			return nil, fmt.Errorf("数据解析失败: %w", err)
		}
		replies = append(replies, &t)
	}

	// 检查遍历错误（网页5）
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("结果集处理错误: %w", err)
	}

	// // 空结果处理（网页2）
	// if len(replies) == 0 {
	// 	return nil, fmt.Errorf("暂无回复")
	// }

	tools.ReverseInPlace(replies)

	return replies, nil
}

func getThread(tid, pn int) (*Thread, error) {
	var wg sync.WaitGroup

	// 设置等待的线程数量
	wg.Add(2) // [1,2,6](@ref)

	var thread *Thread
	var list []*Thread
	var err1 error
	var err2 error
	go func() {
		defer wg.Done() // 线程结束时计数器减1（defer确保执行）

		thread, err1 = getThreadByNo(tid)
	}()
	go func() {
		defer wg.Done() // 线程结束时计数器减1（defer确保执行）

		list, err2 = getReplies(tid, pn)
	}()

	wg.Wait() // [1,2,4](@ref)

	var merr *multierror.Error
	merr = multierror.Append(merr, err1)
	merr = multierror.Append(merr, err2)
	if err := merr.ErrorOrNil(); err != nil {
		return nil, err
	}

	thread.List = list
	thread.Num = -thread.Num

	return thread, nil
}

// 获得board的pn页的回复
func getBoardThreads(bid, pn int) ([]*Thread, error) {
	threads := make([]*Thread, 0, pageSize/2)
	// 使用 Query 获取多行结果（网页5][网页7）
	rows, err := db.Query(
		`SELECT t.t, t.n, t.ts, t.id, t.no, t.p, t.txt, b.replynum 
		FROM board AS b
		INNER JOIN thread AS t
			ON b.tid = t.no
		WHERE b.bid = ? 
			AND t.del >= 0 
		ORDER BY b.last DESC 
		LIMIT ? OFFSET ?`,
		bid, pageSize/2, (pageSize/2)*pn,
	)
	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}
	defer rows.Close() // 确保关闭连接（网页7强调）

	// 遍历结果集（网页5][网页7）
	for rows.Next() {
		var t Thread
		if err := rows.Scan(
			&t.T, &t.N, &t.Ts, &t.ID, &t.No, &t.P, &t.Txt, &t.Num,
		); err != nil {
			return nil, fmt.Errorf("数据解析失败: %w", err)
		}
		threads = append(threads, &t)
	}

	// 检查遍历错误（网页5）
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("结果集处理错误: %w", err)
	}

	return threads, nil
}

func getBoard(bid, pn int) ([]*Thread, error) {
	threads, err := getBoardThreads(bid, pn)
	if err != nil {
		return nil, err
	}

	n := len(threads)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n) // [1,2,6](@ref)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()

			threads[i].List, errs[i] = getRepliesPreview(int(threads[i].No))
		}(i)
	}
	wg.Wait()

	var merr *multierror.Error
	merr = multierror.Append(merr, errs...)
	if err := merr.ErrorOrNil(); err != nil {
		return nil, err
	}
	return threads, err

}

func get(c *gin.Context) {
	bid := tools.Atoi(c.Query("bid"), 0)
	tid := tools.Atoi(c.Query("tid"), 0)
	pn := tools.Atoi(c.Query("pn"), 0)

	if tid == 0 && bid != 0 { // board
		o, e := getBoard(bid, pn)
		if e != nil {
			c.JSON(http.StatusInternalServerError, e.Error())
			return
		}
		c.JSON(http.StatusOK, o)
	} else if tid != 0 {
		o, e := getThread(tid, pn)
		if e != nil {
			c.JSON(http.StatusInternalServerError, e.Error())
			return
		}
		c.JSON(http.StatusOK, o)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plz set tid or bid."})
	}
}

func getMaxNo() (int, error) {
	var maxNo int
	err := db.QueryRow("SELECT MAX(no) FROM thread").Scan(&maxNo)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("获取最大no失败: %w", err)
	}
	return maxNo, nil
}

func postThreadToBoard(bid, tid int) (int, error) {
	// 插入到板中
	result, err := db.Exec(
		"INSERT IGNORE  INTO board (bid, tid) VALUES (?, ?)",
		bid, tid,
	)
	if err != nil {
		return 0, fmt.Errorf("插入到板失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("获取影响行数失败: %w", err)
	}

	return int(rowsAffected), nil
}

func updateReplyNum(tid int) error {
	_, err := db.Exec(
		`UPDATE board 
		SET replynum = (
			SELECT COUNT(*) 
			FROM thread
			WHERE r = ? AND del >= 0
		),
		last = CURRENT_TIMESTAMP()
		WHERE tid = ?`,
		tid, tid,
	)

	return err
}

func postThread(thread *Thread, bid int) (int64, error) {

	// 执行插入
	result, err := db.Exec(
		"INSERT INTO thread (t, n, id, p, txt, r, del, c, ip) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)",
		thread.T, thread.N, thread.ID, thread.P, thread.Txt, thread.R, thread.C, thread.IP,
	)
	if err != nil {
		return -1, fmt.Errorf("插入主题失败: %w", err)
	}

	// 获取插入的ID
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("获取插入ID失败: %w", err)
	}
	thread.No = uint(lastInsertID)

	// 如果指定了bid，则添加到板中
	if thread.R == 0 && bid > 0 {
		_, err = postThreadToBoard(bid, int(lastInsertID))
		if err != nil {
			return -1, fmt.Errorf("添加到板失败: %w", err)
		}
	}

	// 如果是回复，那么更新replynum和lastreply（自动）
	if thread.R != 0 {
		if err := updateReplyNum(int(thread.R)); err != nil {
			return -1, fmt.Errorf("更新回复数失败: %w", err)
		}
	}
	return lastInsertID, nil
}

func post(c *gin.Context) {
	bid := tools.Atoi(c.Query("bid"), 0)
	tid := tools.Atoi(c.Query("tid"), 0)
	// pn := tools.Atoi(c.Query("pn"), 0)

	var thread Thread
	if err := c.BindJSON(&thread); err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
	}

	thread.R = tools.Or(thread.R, uint(tid))

	if thread.R == 0 && bid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plz set tid or bid."})
		return
	}

	thread.ID = c.GetString("id")
	thread.C = c.GetHeader("Cf-Ipcountry")
	thread.IP = c.GetHeader("X-Forwarded-For")

	LastInsertId, err := postThread(&thread, bid)
	if err != nil {
		c.Header("X-Error", err.Error())
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	thread.No = uint(LastInsertId)

	// 送给bot处理
	go func() {
		body, err := json.Marshal(thread)
		if err != nil {
			return
		}
		for _, line := range strings.Split(thread.Txt, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "@") {
				botName, query, err := tools.SeprateString(" ", strings.TrimSpace(line))
				if err == nil {
					go bot.Request(LastInsertId, botName, query, body)
				} else {
					go bot.Request(LastInsertId, strings.TrimSpace(line), "", body)
				}
			}
		}
	}()

	c.AbortWithStatus(http.StatusOK)
}

func deleteThread(no int, id, ip string) error {
	// 执行插入
	_, err := db.Exec(
		"UPDATE thread SET del = -1 WHERE no = ? AND (id = ? OR ip = ?)",
		no, id, ip,
	)
	return err
}

func delete(c *gin.Context) {
	id := c.GetString("id")
	ip := c.GetHeader("X-Forwarded-For")
	no := tools.Atoi(c.Query("no"), 0)

	// var thread Thread
	// if err := c.BindJSON(&thread); err != nil {
	// 	c.JSON(http.StatusBadRequest, err.Error())
	// }

	err := deleteThread(no, id, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.AbortWithStatusJSON(http.StatusOK, gin.H{
		"id": id,
		"ip": ip,
	})
}

func checkID(c *gin.Context) {
	auth, err := c.Cookie("auth")
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	arr := strings.Split(auth, "|")
	id, hash := arr[0], arr[1]
	if tools.Hash(id, os.Getenv("SALT")) != hash {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.Set("id", id)
}

func cookie(c *gin.Context) {
	id := make([]byte, 8)
	if c.Query("id") != "" { // 怎么可能有人猜到
		id = []byte(c.Query("id"))
	} else {
		_, err := randomreader.Read(id)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, err)
			return
		}
	}
	c.SetSameSite(http.SameSiteNoneMode)
	hash := tools.Hash(string(id), os.Getenv("SALT"))
	auth := string(id) + "|" + hash
	c.SetCookie("auth", auth, 3600*24*365*10, "/", "", false, false)
}

// 固定屎upload.moonchan.xyz
func preview(c *gin.Context) {
	cached := false
	// cacheKey :=  path+"?"+query.Encode()

	path := c.Param("path")
	host := tools.Or(c.Query("proxy_host"), c.Query("host"))
	query := c.Request.URL.Query()
	query.Del("host")
	header := tools.NewHeader(c.Request.Header)
	header.Set("Referer", c.Query("proxy_referer"))

	url := "https://" + host + path + "?" + query.Encode()
	if host == "upload.moonchan.xyz" && tools.HasEnv("AZURE") {
		url = "http://" + os.Getenv("AZURE") + path + "?" + query.Encode()
	}
	if v, err := kv.QueryValue(path + "?" + query.Encode()); err == nil && v != "" {
		c.Redirect(http.StatusMovedPermanently, "https://upload.moonchan.xyz/api/"+v+"/thumbnail.jpg")
		return
		// cached = true
	} else if err == nil && v == "" {
		c.Header("X-Cached", "true")
		c.Header("X-Error", "不支持的图片格式")
		c.String(http.StatusBadRequest, "不支持的图片格式")
		return
	}

	resp, err := myfetch.Fetch(http.MethodGet, url, header.Header, nil)
	if err != nil {
		c.Header("X-Error", err.Error())
		c.String(http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	if cached {
		c.Header("X-Cached", "true")
		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
			"Cache-Control": "public, max-age=31536000, immutable",
			"Expires":       time.Now().Add(365 * 24 * time.Hour).Format(http.TimeFormat),
			// "ETag":          v,
			"Last-Modified": "Tue, 27 May 2025 00:00:00 GMT",
		})
		return
	}

	// 不在支持列表。
	if !slices.Contains(
		[]string{"image/jpeg", "image/jpg", "image/webp", "image/png", "image/gif", "image/avif"},
		resp.Header.Get("Content-Type")) {

		c.Header("X-Cached", "false")
		c.Header("X-Content-Type", resp.Header.Get("Content-Type"))
		kv.AddOrUpdate(path+"?"+query.Encode(), "")
		c.Header("X-Error", "不支持的图片格式")
		c.String(http.StatusBadRequest, "不支持的图片格式")
		return
	}

	img, err := tools.DecodeResponseToImage(resp)
	if err != nil {
		c.Header("X-Cached", "false")
		c.Header("X-Content-Type", resp.Header.Get("Content-Type"))
		kv.AddOrUpdate(path+"?"+query.Encode(), "")
		c.Header("X-Error", err.Error())
		c.String(http.StatusBadGateway, err.Error())
		return
	}

	// 4. 生成缩略图（保持宽高比）
	thumbnail := resize.Thumbnail(480, 480, img, resize.Lanczos3)
	go uploadAndCache(thumbnail, path+"?"+query.Encode())
	// 输出JPEG格式
	c.Writer.Header().Set("Content-Type", "image/jpeg")
	err = jpeg.Encode(c.Writer, thumbnail, &jpeg.Options{Quality: 80})
	if err != nil {
		// c.Header("X-Error", err.Error())
		// c.String(http.StatusInternalServerError, err.Error())
		return
	}

}

func Run(addr string) error {
	if addr == "" {
		return nil
	}

	bot.SetDB(tools.Match(bot.NewDB(db)).Result())

	r := gin.Default()

	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	r.POST("/groq", handler.GroqHandler)

	r.GET("/api/v2/", get)
	// r.GET("/api/v2/preview/*path", preview)
	r.GET("/api/v2/cookie", cookie)
	r.POST("/api/v2/", checkID, post)
	r.DELETE("/api/v2/", checkID, delete)
	r.GET("/api/v2/reaction/:tid", checkID, GetReactionsHandlerAlt)
	r.POST("/api/v2/reaction/:tid", checkID, SetReactionHandlerAlt /*, updateNewReaction*/)
	r.GET("/api/v2/new_reactions", getNewReactions)
	// r.GET("/api/v2/reactions", checkID, GetReactionsBatchHandler) // no longer used
	r.GET("/api/v2/cover", getRandomRecordHandler)
	r.GET("/api/v2/bilicover", getRandomRecordHandlerBili)
	r.POST("/api/v2/cover", checkID, addURLHandler)
	r.GET("/api/v2/random", getRandomHandler)
	r.POST("/api/v2/random", addRandomHandler)
	r.GET("/api/v2/bot/:bot/:tid", bot.Handler)
	r.POST("/api/v2/bot/:bot/:tid", func(c *gin.Context) {
		tid := tools.Atoi(c.Param("tid"), 0)
		thread, err := getThreadByNo(tid)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		threadJSON, err := json.Marshal(thread)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Set("thread", string(threadJSON))
	}, bot.Handler)

	r.NoRoute(handler.NoRoute("/var/www/moonchan", "index.html"))

	return r.Run(addr)
}
