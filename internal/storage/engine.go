package storage

import (
	"context"
	"io"
)

type StorageEngine interface {
	Put(key string, r io.Reader, size int64) error
	InitUpload(uploadID string) error
	UploadPart(uploadID string, partNumber int, r io.Reader, size int64) error // 修复：partNumber 是 int
	CompleteUpload(uploadID string, parts []Part) (string, error)
	AbortUpload(uploadID string) error
	UploadStream(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error
}

type Part struct {
	PartNumber int
	ETag       string
}
