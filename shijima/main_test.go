package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"

	tools "github.com/Hana-ame/api-pack/Tools"
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
	tools.SaveStructToJsonFile(ts, "a.json")
}
