// azure-go @ 2023-12-21

package tools

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
)

// this function receive json request.
func ReadJSONFile(fn string) (*orderedmap.OrderedMap, error) {
	jsonFile, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	o := orderedmap.New()
	err = json.NewDecoder(jsonFile).Decode(&o)
	return o, err
}

func ReaderToJSON(reader io.Reader) (*orderedmap.OrderedMap, error) {
	o := orderedmap.New()
	err := json.NewDecoder(reader).Decode(&o)
	return o, err
}

func StringToJSON(s string) (*orderedmap.OrderedMap, error) {
	return ReaderToJSON(strings.NewReader(s))
}

func BytesToJSON(b []byte) (*orderedmap.OrderedMap, error) {
	return ReaderToJSON(bytes.NewReader(b))
}

func FileToJSON(fn string) (*orderedmap.OrderedMap, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	return ReaderToJSON(f)
}

func GzipFileToJSON(fn string) (*orderedmap.OrderedMap, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	return ReaderToJSON(reader)
}

func SaveToJSON(data interface{}, filePath string) error {
	// 1. 先将结构体编码为JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	// 2. 创建目标文件
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// 4. 将JSON数据写入文件
	_, err = file.Write(jsonData)
	if err != nil {
		return fmt.Errorf("GZIP压缩失败: %w", err)
	}

	return nil
}

// SaveToGzip 将结构体数据压缩为GZIP格式并保存到文件
func SaveToGzip(data interface{}, filePath string, level int) error {
	// 1. 先将结构体编码为JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	// 2. 创建目标文件
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// 3. 创建gzip写入器
	gzWriter, err := gzip.NewWriterLevel(file, level)
	if err != nil {
		return fmt.Errorf("压缩level错误: %w", err)
	}
	defer gzWriter.Close()

	// 4. 将JSON数据写入gzip写入器
	_, err = gzWriter.Write(jsonData)
	if err != nil {
		return fmt.Errorf("GZIP压缩失败: %w", err)
	}

	return nil
}

// readJsonFileToStruct 将 JSON 文件读取并反序列化到结构体
func UnmarshalFromFile(filePath string, data interface{}) error {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开 JSON 文件失败: %w", err)
	}
	defer file.Close()

	// 创建 JSON 解码器
	decoder := json.NewDecoder(file)

	// 解码 JSON 数据到结构体
	err = decoder.Decode(data)
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("文件为空或者没有JSON数据: %w", err)
		}
		return fmt.Errorf("JSON 解码失败: %w", err)
	}

	return nil
}

func OrderedMapFromKVArray(kvArray Slice[*orderedmap.Pair]) *orderedmap.OrderedMap {
	o := orderedmap.New()
	for _, kv := range kvArray {
		o.Set(kv.Key(), kv.Value())
	}
	return o
}
