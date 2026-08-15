package r2

import (
	"strconv"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

func (b *Bucket) UploadHandler() func(c *gin.Context) {
	return func(c *gin.Context) {
		defer c.Request.Body.Close()

		key := strconv.Itoa(int(tools.NewTimeStamp()))

		err := b.Upload(key, (c.Request.Body), c.GetHeader("Content-Type"), c.Request.ContentLength) // Upload the file from the request body
		if tools.AbortWithError(c, 500, err) {
			return
		}

		c.JSON(200, gin.H{
			"key": key,
		})
	}
}

func (b *Bucket) DownloadHandler(param string) func(c *gin.Context) {
	return func(c *gin.Context) {
		key := c.Param(param) // Get the key from the URL parameter

		output, err := b.GetObject(key) // Upload the file from the request body
		if tools.AbortWithError(c, 500, err) {
			return
		}
		defer output.Body.Close() // Ensure the body is closed after use

		// // 空值安全处理函数
		// safeStr := func(s *string) string {
		// 	if s == nil {
		// 		return "null"
		// 	}
		// 	return *s
		// }
		// headers := map[string]string{
		// 	// 标准 HTTP Header 映射
		// 	"Accept-Ranges":       safeStr(output.AcceptRanges),
		// 	"Cache-Control":       safeStr(output.CacheControl),
		// 	"Content-Disposition": safeStr(output.ContentDisposition),
		// 	"Content-Encoding":    safeStr(output.ContentEncoding),
		// 	"Content-Language":    safeStr(output.ContentLanguage),
		// 	"Content-Length":      strconv.FormatInt(tools.Unpack(output.ContentLength), 10),
		// 	"Content-Range":       safeStr(output.ContentRange),
		// 	"Content-Type":        safeStr(output.ContentType),
		// 	"ETag":                safeStr(output.ETag),
		// 	"Expires":             safeStr(output.ExpiresString),            // 使用原始字符串值
		// 	"Last-Modified":       output.LastModified.Format(time.RFC1123), // RFC1123 格式[3](@ref)

		// 	// S3 校验和 Header
		// 	"x-amz-checksum-crc32":     safeStr(output.ChecksumCRC32),
		// 	"x-amz-checksum-crc32c":    safeStr(output.ChecksumCRC32C),
		// 	"x-amz-checksum-crc64nvme": safeStr(output.ChecksumCRC64NVME),
		// 	"x-amz-checksum-sha1":      safeStr(output.ChecksumSHA1),
		// 	"x-amz-checksum-sha256":    safeStr(output.ChecksumSHA256),
		// 	"x-amz-checksum-type":      string(output.ChecksumType),

		// 	// S3 特有元数据
		// 	"x-amz-bucket-key-enabled":     strconv.FormatBool(tools.Unpack(output.BucketKeyEnabled)),
		// 	"x-amz-delete-marker":          strconv.FormatBool(tools.Unpack(output.DeleteMarker)),
		// 	"x-amz-expiration":             safeStr(output.Expiration),
		// 	"x-amz-object-lock-hold":       string(output.ObjectLockLegalHoldStatus),
		// 	"x-amz-object-lock-mode":       string(output.ObjectLockMode),
		// 	"x-amz-object-lock-until":      tools.Unpack(output.ObjectLockRetainUntilDate).Format(time.RFC3339),
		// 	"x-amz-replication-status":     string(output.ReplicationStatus),
		// 	"x-amz-request-charged":        string(output.RequestCharged),
		// 	"x-amz-restore":                safeStr(output.Restore),
		// 	"x-amz-server-side-encryption": string(output.ServerSideEncryption),
		// 	"x-amz-storage-class":          string(output.StorageClass),
		// 	"x-amz-tag-count":              strconv.FormatInt(int64(tools.Unpack(output.TagCount)), 10),
		// 	"x-amz-version-id":             safeStr(output.VersionId),
		// 	"Location":                     safeStr(output.WebsiteRedirectLocation), // 重定向特殊处理

		// 	// 加密相关 Header
		// 	"x-amz-server-side-encryption-customer-algorithm": safeStr(output.SSECustomerAlgorithm),
		// 	"x-amz-server-side-encryption-customer-key-MD5":   safeStr(output.SSECustomerKeyMD5),
		// 	"x-amz-server-side-encryption-aws-kms-key-id":     safeStr(output.SSEKMSKeyId),
		// }

		// // 添加自定义元数据 (x-amz-meta-前缀)[6](@ref)
		// for k, v := range output.Metadata {
		// 	headers["x-amz-meta-"+k] = v
		// }

		c.DataFromReader(200, *(output.ContentLength), *(output.ContentType), output.Body, map[string]string{
			"Content-Disposition": "inline",
		})
	}
}
