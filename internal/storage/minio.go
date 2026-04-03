package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStorage struct {
	client *minio.Client
	bucket string
}

// NewMinIOStorage 初始化MinIO存储引擎
func NewMinIOStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// 检查bucket是否存在，不存在则创建
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: "us-east-1"})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &MinIOStorage{
		client: client,
		bucket: bucket,
	}, nil
}

// Put 上传单个文件
func (m *MinIOStorage) Put(key string, r io.Reader, size int64) error {
	ctx := context.Background()
	_, err := m.client.PutObject(ctx, m.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}

// InitUpload 初始化分片上传
func (m *MinIOStorage) InitUpload(uploadID string) error {
	// MinIO提供类似S3的分片上传，但不需要主动初始化
	// 这里我们可以创建一个标记文件或记录在数据库中
	// 跳过即可，真实场景中应该记录uploadID到数据库
	return nil
}

// UploadPart 上传单个分片
// partNumber从1开始
func (m *MinIOStorage) UploadPart(uploadID string, partNumber int, r io.Reader, size int64) error {
	ctx := context.Background()
	partKey := filepath.Join("temp", uploadID, fmt.Sprintf("part-%d", partNumber))
	_, err := m.client.PutObject(ctx, m.bucket, partKey, r, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}

// CompleteUpload 完成分片上传，合并所有分片
func (m *MinIOStorage) CompleteUpload(uploadID string, parts []Part) (string, error) {
	ctx := context.Background()
	finalKey := uploadID + ".merged"

	// 获取所有分片并合并
	var buf bytes.Buffer
	for _, part := range parts {
		partKey := filepath.Join("temp", uploadID, fmt.Sprintf("part-%d", part.PartNumber))
		obj, err := m.client.GetObject(ctx, m.bucket, partKey, minio.GetObjectOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to get part %d: %w", part.PartNumber, err)
		}
		defer obj.Close()

		if _, err := io.Copy(&buf, obj); err != nil {
			return "", fmt.Errorf("failed to read part %d: %w", part.PartNumber, err)
		}

		// 删除分片文件
		if err := m.client.RemoveObject(ctx, m.bucket, partKey, minio.RemoveObjectOptions{}); err != nil {
			// 记录错误但继续，防止阻塞
			fmt.Printf("warning: failed to remove part %s: %v\n", partKey, err)
		}
	}

	// 上传合并后的文件
	_, err := m.client.PutObject(ctx, m.bucket, finalKey, &buf, int64(buf.Len()), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload merged file: %w", err)
	}

	// 删除临时目录
	// 注意：MinIO不直接支持删除目录，需要逐个删除
	// 这里简化处理，实际场景可以记录临时文件列表后删除

	return finalKey, nil
}

// AbortUpload 中止分片上传
func (m *MinIOStorage) AbortUpload(uploadID string) error {
	ctx := context.Background()
	tempDir := filepath.Join("temp", uploadID)

	// 列出临时目录下的所有文件并删除
	objectsChan := m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{
		Prefix:    tempDir,
		Recursive: true,
	})

	for object := range objectsChan {
		if object.Err != nil {
			return object.Err
		}
		if err := m.client.RemoveObject(ctx, m.bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (m *MinIOStorage) UploadStream(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, m.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}
