package storage

import (
	"io"
)

type StorageEngine interface {
	// Put 将数据流写入存储，返回写入的字节数和可能的错误
	Put(key string, r io.Reader, size int64) error
}
