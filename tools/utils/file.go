package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// WriteJSONToFile 将数据序列化为 JSON 并写入指定文件
func WriteJSONToFile(filename string, data any) error {
	// 将数据序列化为格式化的 JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化为 JSON 时出错: %w", err)
	}

	// 打开或创建文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件时出错: %w", err)
	}
	defer file.Close() // 确保在函数结束时关闭文件

	// 写入 JSON 数据到文件
	_, err = file.Write(jsonData)
	if err != nil {
		return fmt.Errorf("写入文件时出错: %w", err)
	}

	return nil
}

func WriteReaderToFile(filename string, reader io.Reader) (err error) {
	f, err := os.Create(filename) // 可以create同一个文件名的
	if err != nil {
		return
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	_, err = io.Copy(w, reader)
	return
}

func WriteStringToFile(filename, s string) (err error) {
	reader := strings.NewReader(s)
	return WriteReaderToFile(filename, reader)
}

func WriteDataToFile(filename string, data []byte) (err error) {
	reader := bytes.NewReader(data)
	return WriteReaderToFile(filename, reader)
}
