package storage

import (
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	BaseDir string
}

// internal/storage/engine.go (接口定义)
// type StorageEngine interface {
//     // 使用 io.Reader 实现流式写入，防止大文件导致 OOM (Out Of Memory)
//     Put(key string, r io.Reader, size int64) error
// }

func NewLocalStorage(baseDir string) *LocalStorage {
	os.MkdirAll(baseDir, 0755)
	return &LocalStorage{BaseDir: baseDir}
}

func (l *LocalStorage) Put(key string, r io.Reader) (int64, error) {
	fullPath := filepath.Join(l.BaseDir, key)

	// 打开文件（如果不存在则创建，如果存在则截断）
	f, err := os.Create(fullPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return io.Copy(f, r) // 核心 IO 操作
}

func (l *LocalStorage) Get(key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(l.BaseDir, key)
	return os.Open(fullPath)
}

func (l *LocalStorage) Delete(key string) error {
	fullPath := filepath.Join(l.BaseDir, key)
	return os.Remove(fullPath)
}

func (l *LocalStorage) Exists(key string) (bool, error) {
	fullPath := filepath.Join(l.BaseDir, key)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
