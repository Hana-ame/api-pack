package shijima

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	"github.com/Hana-ame/api-pack/tools/orderedmap"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/nfnt/resize"
)

func TestGetThreadByNo(t *testing.T) {
	tests := []struct {
		name     string // 测试用例名称
		threadNo int    // 输入参数
		wantErr  bool   // 预期是否报错
	}{
		{
			name:     "正常存在的帖子",
			threadNo: 143736,
			wantErr:  false,
		},
		{
			name:     "不存在的帖子编号",
			threadNo: 143737,
			wantErr:  true,
		},
		{
			name:     "非法参数（负数）",
			threadNo: -100,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getThreadByNo(tt.threadNo)

			// 错误断言
			if (err != nil) != tt.wantErr {
				t.Errorf("getThreadByNo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			log.Println(err)

			// 非错误场景校验内容
			if !tt.wantErr {
				if got == nil {
					t.Error("预期返回有效对象，实际为nil")
				}
				log.Println(string(tools.Match(json.Marshal(got)).Result()))
			}
		})
	}
}

func TestGetRepliesPreview(t *testing.T) {
	tests := []struct {
		name     string // 测试用例名称
		threadNo int    // 输入参数
	}{
		{
			name:     "没有rep",
			threadNo: 143758,
		},
		{
			name:     "两个rep",
			threadNo: 143748,
		},
		{
			name:     "老多rep",
			threadNo: 125564,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, e := getRepliesPreview(tt.threadNo)
			fmt.Println(e)
			fmt.Println(string(tools.Match(json.Marshal(r)).Result()))
		})
	}
}

func TestGetReplies(t *testing.T) {
	tests := []struct {
		name     string // 测试用例名称
		threadNo int    // 输入参数
		pn       int
	}{
		{
			name:     "pn0",
			threadNo: 125564,
			pn:       0,
		},
		{
			name:     "pn1",
			threadNo: 125564,
			pn:       1,
		},
		{
			name:     "pn99",
			threadNo: 125564,
			pn:       99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, e := getReplies(tt.threadNo, tt.pn)
			fmt.Println(e)
			fmt.Println(string(tools.Match(json.Marshal(r)).Result()))
		})
	}
}

func TestGetThread(t *testing.T) {
	tests := []struct {
		name     string // 测试用例名称
		threadNo int    // 输入参数
		pn       int
	}{
		{
			name:     "pn0",
			threadNo: 125564,
			pn:       0,
		},
		{
			name:     "pn1",
			threadNo: 125564,
			pn:       1,
		},
		{
			name:     "not exist",
			threadNo: 100,
			pn:       99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, e := getThread(tt.threadNo, tt.pn)
			fmt.Println(e)
			fmt.Println(string(tools.Match(json.Marshal(r)).Result()))
		})
	}
}

func TestExample(t *testing.T) {
	// 数据库连接配置
	dsn := os.Getenv("MARIADB")

	// 建立连接
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}
	defer db.Close()

	// 执行查询
	rows, err := db.Query(`
        SELECT 
            t, n, ts, id, no, p, txt, r, del, c, ip 
        FROM 
            thread
		LIMIT 10;
    `)
	if err != nil {
		log.Fatal("查询失败:", err)
	}
	defer rows.Close()

	// 遍历结果集
	var results []Thread
	for rows.Next() {
		var t Thread
		err := rows.Scan(
			&t.T,
			&t.N,
			&t.Ts,
			&t.ID,
			&t.No,
			&t.P,
			&t.Txt,
			&t.R,
			&t.Del,
			&t.C,
			&t.IP,
		)
		if err != nil {
			log.Fatal("数据解析失败:", err)
		}
		results = append(results, t)
	}

	// 输出结果
	fmt.Printf("共查询到 %d 条记录\n", len(results))
	for _, thread := range results {
		fmt.Printf("ID:%s 时间:%s 内容:%.20s...\n",
			thread.ID,
			thread.Ts,
			thread.Txt)
	}
}

func TestBoard(t *testing.T) {
	ts, er := getBoard(1, 0)
	fmt.Println(er)
	fmt.Println(string(tools.Match(json.Marshal(ts)).Result()))
	tools.SaveToJSON(ts, "a.json")
}

func TestPostThread(t *testing.T) {
	fmt.Println("version", 1)
	// 测试用例表格
	tests := []struct {
		name        string
		thread      *Thread
		bid         int
		wantErr     bool
		wantNo      uint
		postToBoard bool
	}{
		{
			name: "成功插入主题并添加到板",
			thread: &Thread{
				T:   "测试标题",
				N:   "测试用户",
				ID:  "test123",
				P:   "test.jpg",
				Txt: "测试内容",
				R:   0,
				C:   "CN",
				IP:  "127.0.0.1",
			},
			bid:         1,
			wantErr:     false,
			wantNo:      100,
			postToBoard: true,
		},
		{
			name: "成功插入回复主题",
			thread: &Thread{
				T:   "",
				N:   "回复用户",
				ID:  "reply123",
				P:   "",
				Txt: "回复内容",
				R:   100,
				C:   "US",
				IP:  "192.168.1.1",
			},
			bid:         0,
			wantErr:     false,
			wantNo:      101,
			postToBoard: false,
		},
		{
			name: "数据库插入失败",
			thread: &Thread{
				T:   "失败测试",
				N:   "测试用户",
				ID:  "test123",
				Txt: "测试内容",
				R:   0,
			},
			bid:         1,
			wantErr:     true,
			postToBoard: false,
		},
		{
			name: "添加到板失败",
			thread: &Thread{
				T:   "测试标题",
				N:   "测试用户",
				ID:  "test123",
				Txt: "测试内容",
				R:   0,
			},
			bid:         1,
			wantErr:     true,
			wantNo:      100,
			postToBoard: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 保存原始db对象以便测试后恢复

			// 模拟postThreadToBoard函数
			_, err := postThread(tt.thread, 2)
			fmt.Println(err)

		})
	}
}

func TestDel(t *testing.T) {
	URL, _ := url.Parse("/api/v2/preview/media/GrfZh0daUAAhFwi?format=jpg&name=small&host=pbs.twimg.com")
	URL.Query().Del("host")
	URL.Query().Encode()
	fmt.Println(URL.Query().Encode())
}

func TestPreview(t *testing.T) {

	path := "/preview/media/GrfZh0daUAAhFwi"
	host := "pbs.twimg.com"
	// c.Request.URL.Query().Del("host")
	header := tools.NewHeader(nil)
	// header.Add("Referer", c.Query("proxy_referer"))

	URL, _ := url.Parse("/api/v2/preview/media/GrfZh0daUAAhFwi?format=jpg&name=small&host=pbs.twimg.com")
	URL.Query().Del("host")

	resp, err := myfetch.Fetch(http.MethodGet, "https://"+host+path+"?"+URL.Query().Encode(), header.Header, nil)
	if err != nil {
		// c.Header("X-Error", err.Error())
		// c.String(http.StatusBadGateway, err.Error())
		fmt.Println(err)
		return
	}

	img, err := tools.DecodeResponseToImage(resp)
	if err != nil {
		// c.Header("X-Error", err.Error())
		// c.String(http.StatusBadGateway, err.Error())
		fmt.Println(err)
		return
	}

	// 4. 生成缩略图（保持宽高比）
	thumbnail := resize.Thumbnail(320, 320, img, resize.Lanczos3)

	// 输出JPEG格式
	// c.Writer.Header().Set("Content-Type", "image/jpeg")
	// err = jpeg.Encode(c.Writer, thumbnail, &jpeg.Options{Quality: 80})
	if err != nil {
		// c.Header("X-Error", err.Error())
		// c.String(http.StatusInternalServerError, err.Error())
		fmt.Println(err)
	}
	_ = thumbnail
}

// simulateImageCreation 模拟生成一个图像（例如，一个红色方块）
func simulateImageCreation() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// 填充红色
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	return img
}
func TestUploadAndCache(t *testing.T) {
	thumbnail := simulateImageCreation() // 得到你的图像数据
	err := uploadAndCache(thumbnail, "123")
	log.Println(err)
}

func TestAlt(t *testing.T) {

	// 确保表存在
	if err := createTableIfNotExists(); err != nil {
		log.Fatalf("Error ensuring table exists: %v", err)
	}

	fmt.Println("\n--- Testing setReactionAlt ---")
	// 模拟一些操作
	testTID := 123
	err := setReactionAlt(testTID, "👍") // count=1, timestamp=t1
	if err != nil {
		log.Printf("Error setting reaction: %v", err)
	}
	time.Sleep(10 * time.Millisecond)  // 模拟时间流逝，确保timestamp有差异
	err = setReactionAlt(testTID, "👍") // count=2, timestamp=t2
	if err != nil {
		log.Printf("Error setting reaction: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = setReactionAlt(testTID, "❤️") // count=1, timestamp=t3
	if err != nil {
		log.Printf("Error setting reaction: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = setReactionAlt(testTID, "😂") // count=1, timestamp=t4
	if err != nil {
		log.Printf("Error setting reaction: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = setReactionAlt(testTID, "👍") // count=3, timestamp=t5
	if err != nil {
		log.Printf("Error setting reaction: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = setReactionAlt(testTID, "❤️") // count=2, timestamp=t6 (count=2, t6 > t3)
	if err != nil {
		log.Printf("Error setting reaction: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = setReactionAlt(testTID, "😂") // count=2, timestamp=t7 (count=2, t7 > t4)
	if err != nil {
		log.Printf("Error setting reaction: %v", err)
	}
	fmt.Println("Reactions set.")

	fmt.Println("\n--- Testing getReactions ---")
	// 获取并打印结果
	reactionsMap, err := getReactionsAlt(testTID)
	if err != nil {
		log.Fatalf("Error getting reactions: %v", err)
	}

	fmt.Printf("Reactions for tid %d (ordered by count DESC, timestamp DESC):\n", testTID)
	// 遍历有序映射并打印
	for _, key := range reactionsMap.Keys() {
		value, _ := reactionsMap.Get(key)
		fmt.Printf("- Reaction: %s, Count: %d\n", key, value)
	}
	// Expected order:
	// - 👍 (count 3)
	// - 😂 (count 2, because its timestamp is more recent than ❤️'s timestamp for count 2)
	// - ❤️ (count 2)
	// If timestamps are too close to register difference, the order for count=2 might vary.
	// Running with `time.Sleep(10 * time.Millisecond)` should make timestamps distinct enough.
	fmt.Println(string(tools.Match(json.Marshal(reactionsMap)).Result()))

	// Test with another tid
	fmt.Println("\n--- Testing another TID ---")
	testTID2 := 456
	_ = setReactionAlt(testTID2, "😂")
	_ = setReactionAlt(testTID2, "😂")
	_ = setReactionAlt(testTID2, "👍")
	reactionsMap2, err := getReactionsAlt(testTID2)
	if err != nil {
		log.Fatalf("Error getting reactions for tid %d: %v", testTID2, err)
	}
	fmt.Printf("Reactions for tid %d:\n", testTID2)
	for _, key := range reactionsMap2.Keys() {
		value, _ := reactionsMap2.Get(key)
		fmt.Printf("- Reaction: %s, Count: %d\n", key, value)
	}
	fmt.Println(string(tools.Match(json.Marshal(reactionsMap2)).Result()))

}

func TestAlt2(t *testing.T) {
	setReactionAlt(4, "😂")  // count=1, timestamp=t1
	setReactionAlt(3, "❤️") // count=1, timestamp=t1
	setReactionAlt(2, "👍")  // count=1, timestamp=t1
	fmt.Println([]byte("😂"))
	fmt.Println([]byte("👍"))
	fmt.Println([]byte("❤️"))

}

func TestSp(t *testing.T) {
	a, b, c := tools.SeprateString(" ", "@reaction test")
	fmt.Println(a, b, c)
	arr := []byte{123, 34, 116, 115, 34, 58, 34, 34, 44, 34, 105, 100, 34, 58, 34, 68, 89, 52, 88, 53, 73, 72, 82, 34, 44, 34, 110, 111, 34, 58, 49, 52, 52, 56, 51, 54, 44, 34, 116, 120, 116, 34, 58, 34, 64, 114, 101, 97, 99, 116, 105, 111, 110, 32, 116, 101, 115, 116, 92, 110, 34, 44, 34, 114, 34, 58, 49, 52, 52, 56, 49, 49, 125}
	fmt.Println(string(arr))
	om := orderedmap.New()
	if err := json.Unmarshal(arr, &om); err != nil {
	}
	j, _ := json.Marshal(om)
	fmt.Println(string(j))
	r := int(om.GetOrDefault("r", float64(0)).(float64))
	fmt.Println(r)
	if r == 0 {
	}
	myfetch.Fetch(http.MethodPost, "https://moonchan.xyz/api/v2/reaction/"+strconv.Itoa(int(r)), http.Header{"Content-Type": []string{"plain/text"}}, strings.NewReader("query"))

}
