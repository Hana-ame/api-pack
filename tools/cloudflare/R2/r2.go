package r2

import (
	"context"
	"fmt"
	"io"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Bucket struct {
	config struct {
		name            string
		accountId       string
		accessKeyId     string
		accessKeySecret string
	}
	*s3.Client
}

func NewBucket(name, accountId, accessKeyId, accessKeySecret string) (*Bucket, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId))
	})

	return &Bucket{
		Client: client,
		config: struct {
			name            string
			accountId       string
			accessKeyId     string
			accessKeySecret string
		}{
			name:            name,
			accountId:       accountId,
			accessKeyId:     accessKeyId,
			accessKeySecret: accessKeySecret,
		},
	}, nil
}

func (b *Bucket) Upload(key string, file io.Reader, contentType string, contentLength int64) error {
	_, err := b.Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:        &b.config.name,
		Key:           &key,
		Body:          file,
		ContentType:   aws.String(tools.Or(contentType, "application/octet-stream")),
		ContentLength: &contentLength,
	})
	return err
}

func (b *Bucket) Download(key string) (io.ReadCloser, error) {
	out, err := b.Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: &b.config.name,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}

	return out.Body, nil
}

func (b *Bucket) GetObject(key string) (*s3.GetObjectOutput, error) {
	return b.Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: &b.config.name,
		Key:    &key,
	})
}

func (b *Bucket) Delete(key string) error {
	_, err := b.Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: &b.config.name,
		Key:    &key,
	})
	return err
}

// 傻逼 s3 在input没有指定Content-Length时是通过 Seek 来测算 Content-Length 的
// 步骤：1.通过(0,1)得到当前位置 2.通过(0,2)得到最后位置 3.通过(n,0)还原1中得到的位置
