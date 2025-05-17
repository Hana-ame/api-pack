package main

import (
	"encoding/json"
	"fmt"
	"log"
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
