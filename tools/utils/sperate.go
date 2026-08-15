package tools

import (
	"bytes"
	"fmt"
)

func Seprate(separator, data []byte) ([]byte, []byte, error) {
	// 查找 \r\n\r\n 的位置
	index := bytes.Index(data, separator)
	if index == -1 {
		err := fmt.Errorf("%s", data)
		return data, data, err
	}
	return data[:index], data[index+len(separator):], nil
}

func SeprateString(separator, data string) (string, string, error) {
	front, end, err := Seprate([]byte(separator), []byte(data))
	return string(front), string(end), err
}
