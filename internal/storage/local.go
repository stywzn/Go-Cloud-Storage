package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		panic("cannot create storage directory: " + err.Error())
	}
	return &LocalStorage{basePath: basePath}
}

// Put 将数据流写入存储，返回写入的字节数和可能的错误
func (s *LocalStorage) Put(key string, r io.Reader, size int64) error {
	fullPath := filepath.Join(s.basePath, key)
	out, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

func (s *LocalStorage) InitUpload(uploadID string) error {
	return os.MkdirAll(filepath.Join(s.basePath, uploadID), 0755)
}

// ... 修复 UploadPart 方法
func (s *LocalStorage) UploadPart(uploadID string, partNumber int, r io.Reader, size int64) error {
	tempDir := filepath.Join(s.basePath, "temp", uploadID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return err
	}
	partPath := filepath.Join(tempDir, fmt.Sprintf("part-%d", partNumber))
	out, err := os.Create(partPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

// ... 修复 CompleteUpload 方法
func (s *LocalStorage) CompleteUpload(uploadID string, parts []Part) (string, error) {
	finalKey := uploadID + ".merged"
	finalPath := filepath.Join(s.basePath, finalKey)
	out, err := os.Create(finalPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	for _, part := range parts {
		partPath := filepath.Join(s.basePath, "temp", uploadID, fmt.Sprintf("part-%d", part.PartNumber))
		in, err := os.Open(partPath)
		if err != nil {
			return "", err
		}
		io.Copy(out, in)
		in.Close()
		os.Remove(partPath)
	}
	os.RemoveAll(filepath.Join(s.basePath, "temp", uploadID))
	return finalKey, nil
}

func (s *LocalStorage) AbortUpload(uploadID string) error {
	return os.RemoveAll(filepath.Join(s.basePath, "temp", uploadID))
}

func (s *LocalStorage) DeleteObject(ctx context.Context, objectName string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fullPath := filepath.Join(s.basePath, objectName)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *LocalStorage) DeletePrefix(ctx context.Context, prefix string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fullPath := filepath.Join(s.basePath, prefix)
	if err := os.RemoveAll(fullPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *LocalStorage) UploadStream(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fullPath := filepath.Join(s.basePath, objectName)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	out, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return err
	}
	return nil
}
