package storage

import (
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
