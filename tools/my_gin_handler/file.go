// 24-09-21 @ gin-pack

package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

// curl -x "" -X PUT http://127.24.7.29:8080/api/file/upload -T README.md
// {"id":113794146377662464,"path":"113/794/146377662464"}
// curl -x "" http://127.24.7.29:8080/api/file/113/794/146377662464/README.md -v
type FileServer struct {
	// 文件保存的位置，是前缀
	Path string
}

type FileMetaData struct {
	ID       int64
	MIMEType string
	FileName string
	Size     int64
}

func (s *FileServer) Upload(c *gin.Context) {

	id := tools.NewTimeStamp()

	idString := strconv.Itoa(int(id))
	pathArray := []string{idString[0:3], idString[3:6], idString[6:]}
	path := strings.Join(pathArray, "/")
	if err := os.MkdirAll(filepath.Join(s.Path, pathArray[0], pathArray[1]), 0755); err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	extensions, _ := mime.ExtensionsByType(tools.Or(c.ContentType(), "application/octet-stream"))

	metaData := &FileMetaData{
		ID:       id,
		MIMEType: tools.Or(c.ContentType(), "application/octet-stream"),
		FileName: idString + tools.NewSlice(extensions...).FirstUnequal(""),
		Size:     c.Request.ContentLength,
	}

	if err := tools.SaveToJSON(metaData, filepath.Join(s.Path, pathArray[0], pathArray[1], pathArray[2]+".metadata.json")); err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	gzipFile, err := os.Create(filepath.Join(s.Path, pathArray[0], pathArray[1], metaData.FileName))
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer gzipFile.Close()

	// gzipWriter := gzip.NewWriter(gzipFile)
	// defer gzipWriter.Close()
	gzipWriter := gzipFile // 跳过压缩，没用

	// 使用 io.Copy 将输入文件内容写入到 gzip 压缩写入器中
	n, err := io.Copy(gzipWriter, c.Request.Body)
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if n != c.Request.ContentLength {
		c.Header("X-Error", fmt.Sprintf("n = %d, expected %d", n, metaData.Size))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"id":   id,
		"path": path,
	})
}

func (s *FileServer) Get(c *gin.Context) {
	pathArray := strings.Split(c.Param("path"), "/")
	if len(pathArray) < 4 {
		c.Header("X-Error", fmt.Sprintf("len(pathArray)  = %d, expected 3 or 4", len(pathArray)))
		c.AbortWithStatus(http.StatusNotFound)
	}

	metadata := new(FileMetaData)
	if err := tools.UnmarshalFromFile(filepath.Join(s.Path, pathArray[1], pathArray[2], pathArray[3]+".metadata.json"), &metadata); err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	gzipFile, err := os.Open(filepath.Join(s.Path, pathArray[1], pathArray[2], metadata.FileName))
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer gzipFile.Close()

	// gzipReader, err := gzip.NewReader(gzipFile)
	// if err != nil {
	// 	c.Header("X-Error", err.Error())
	// 	c.AbortWithStatus(http.StatusInternalServerError)
	// 	return
	// }
	// defer gzipReader.Close()
	gzipReader := gzipFile // 跳过压缩，没用

	c.DataFromReader(http.StatusOK, metadata.Size, metadata.MIMEType, gzipReader, map[string]string{"Content-Disposition": "inline"})
}

// servefile
func FileHandler(filepath func() string) func(c *gin.Context) {
	return func(c *gin.Context) {
		ip := c.GetHeader("CF-Connecting-IP")
		log.Println(ip)
		c.File(filepath())
	}
}

func UploadFile(c *gin.Context) {
	file, err := os.Create("upload/file.txt")
	if err != nil {
		c.JSON(500, gin.H{
			"message": "创建文件失败",
		})
		return
	}
	defer file.Close()

	_, err = io.Copy(file, c.Request.Body)
	if err != nil {
		c.JSON(500, gin.H{
			"message": "写入文件失败",
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "文件上传成功",
	})
}

// UploadFiles 处理文件上传并将文件保存到 /uploads/[sha1sum]/[filename]
func UploadFiles(c *gin.Context) {
	// 获取所有上传的文件
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取文件失败: " + err.Error()})
		return
	}

	// 获取文件列表
	files := form.File["files"] // "files" 是表单中 file input 的 name 属性，可以包含多个文件

	var savedFiles []string

	// 遍历文件并保存
	for _, file := range files {
		// 计算文件的 SHA1 校验和
		fileContent, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "打开文件失败: " + err.Error()})
			return
		}
		defer fileContent.Close()

		hasher := sha1.New()
		if _, err := io.Copy(hasher, fileContent); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "计算 SHA1 失败: " + err.Error()})
			return
		}

		// 获取 SHA1 值并生成新的文件夹路径
		sha1sum := hex.EncodeToString(hasher.Sum(nil))
		dirPath := path.Join("uploads", sha1sum) // 使用 SHA1 值作为文件夹名

		// 创建目录
		if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败: " + err.Error()})
			return
		}

		// 保存文件
		savePath := path.Join(dirPath, file.Filename) // 完整的文件保存路径
		if err := c.SaveUploadedFile(file, savePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败: " + err.Error()})
			return
		}

		savedFiles = append(savedFiles, savePath)
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{"message": "文件上传成功", "files": savedFiles})
}

func File(path string) func(c *gin.Context) {
	// fileInfo, err := os.Stat(path)
	// mimeType := mime.TypeByExtension(filepath.Ext(path)) // 输出: image/jpeg
	return func(c *gin.Context) {
		// 获取文件信息
		// if err != nil {
		// 	c.Header("X-Error", err.Error())
		// 	c.AbortWithStatus(http.StatusNotFound)
		// 	return
		// }

		// 确保文件存在且可读
		if _, err := os.Stat(path); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}

		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		c.Header("X-Frame-Options", "")
		c.Header("X-Frame-Options", "ALLOW-FROM "+origin)
		c.Header("Content-Security-Policy", "frame-src "+origin)

		c.File(path) // "X-Frame-Options” 一直 DENY

		// fileReader, err := os.Open(path)
		// if err != nil {
		// 	c.Header("X-Error", err.Error())
		// 	c.AbortWithStatus(http.StatusNotFound)
		// 	return
		// }
		// defer fileReader.Close()

		// c.DataFromReader(http.StatusOK, fileInfo.Size(), mimeType, fileReader, map[string]string{"Content-Disposition": "inline"})

	}
}
