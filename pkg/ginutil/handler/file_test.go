// 25-01-09 gimini

package handler

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"testing"
)

// 完全没问题
func TestGzip(t *testing.T) {
	compressFile("echo.go", "echo.go.gzip")
	decompressFile("echo.go.gzip", "echo.go")
}

// compressFile 函数用于压缩文件
func compressFile(inputPath, outputPath string) error {
	// 打开输入文件
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("打开输入文件失败: %w", err)
	}
	defer inputFile.Close()

	// 创建输出文件
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outputFile.Close()

	// 创建 gzip 压缩写入器
	gzipWriter := gzip.NewWriter(outputFile)
	defer gzipWriter.Close()

	// 使用 io.Copy 将输入文件内容写入到 gzip 压缩写入器中
	_, err = io.Copy(gzipWriter, inputFile)
	if err != nil {
		return fmt.Errorf("复制文件内容到 gzip 压缩写入器失败: %w", err)
	}

	// 需要手动刷新缓冲区，保证数据写入
	err = gzipWriter.Flush()
	if err != nil {
		return fmt.Errorf("gzip压缩器刷新缓冲区失败: %w", err)
	}
	return nil
}

// decompressFile 函数用于解压缩文件
func decompressFile(inputPath, outputPath string) error {
	// 打开输入文件
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("打开输入文件失败: %w", err)
	}
	defer inputFile.Close()

	// 创建 gzip 读取器
	gzipReader, err := gzip.NewReader(inputFile)
	if err != nil {
		return fmt.Errorf("创建 gzip 读取器失败: %w", err)
	}
	defer gzipReader.Close()

	// 创建输出文件
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outputFile.Close()

	// 使用 io.Copy 将 gzip 解压后的内容写入到输出文件
	_, err = io.Copy(outputFile, gzipReader)
	if err != nil {
		return fmt.Errorf("复制 gzip 解压缩内容到输出文件失败: %w", err)
	}

	return nil
}
