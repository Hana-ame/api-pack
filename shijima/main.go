package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/Hana-ame/api-pack/Tools"
	tools "github.com/Hana-ame/api-pack/Tools"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-multierror"
)

var db, _ = func() (*sql.DB, error) {
	dsn := os.Getenv("MARIADB")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// 连接池参数
	db.SetMaxOpenConns(20)                  // 最大活跃连接数
	db.SetMaxIdleConns(5)                   // 最大空闲连接数
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
	R    uint      `db:"r" json:"-"`            // reply to
	Del  int8      `db:"del" json:"-"`          // is deleted?
	C    string    `db:"c" json:"-"`            // country
	IP   string    `db:"ip" json:"-"`           // ip address
	Num  uint      `json:"num,omitempty"`       // from board
	List []*Thread `json:"list,omitempty"`      // replies
}

// 表结构对应的结构体（根据图片中的列定义）
type Board struct {
	TID      uint `db:"tid" json:"tid,omitempty"`
	BID      uint `db:"bid" json:"bid,omitempty"`
	ReplyNum uint `db:"num"`
}

func getThreadByNo(no int) (*Thread, error) {
	var thread Thread
	thread.No = uint(no)
	err := db.QueryRow(
		`SELECT t,n,ts,id,p,txt
		FROM thread
		WHERE no = ? AND del >= 0;`,
		no,
	).Scan(
		&thread.T, &thread.N, &thread.Ts, &thread.ID, &thread.P, &thread.Txt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("主题不存在")
		}
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}
	return &thread, nil
}

func getReplies(no, pn int) ([]*Thread, error) {
	replies := make([]*Thread, 0, pageSize)

	offset := pn * pageSize

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
	return thread, nil
}

func getBoardThreads(bid, pn int) ([]*Thread, error) {
	threads := make([]*Thread, 0, pageSize)
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
		bid, pageSize, pageSize*pn,
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
	fmt.Println(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()

			threads[i].List, errs[i] = getRepliesPreview(int(threads[i].No))
			fmt.Println(i, string(tools.Match(json.Marshal(threads[i])).Result()))
			tools.SaveStructToJsonFile(threads[i], strconv.Itoa(i)+".json")
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

	if tid == 0 { // board
		o, e := getBoard(bid, pn)
		if e != nil {
			c.JSON(http.StatusInternalServerError, e.Error())
		}
		c.JSON(http.StatusOK, o)
	} else if bid == 0 {
		o, e := getThread(tid, pn)
		if e != nil {
			c.JSON(http.StatusInternalServerError, e.Error())
		}
		c.JSON(http.StatusOK, o)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plz set tid or bid."})
	}
}

func postThreadToBoard(bid, tid int) (int, error) {
	//TODO
	// 更新replynum
}

func postThread(thread *Thread, bid, tid int) error {
	// TODO
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

	get(c)
}

func Run(addr ...string) error {
	r := gin.Default()

	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	r.GET("/api/v2", get)
	r.POST("/api/v2", post)
	// r.DELETE("/api/v2", delete)

	return r.Run(addr...)
}
