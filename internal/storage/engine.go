package storage

import (
	"io"
)

// StorageEngine 定义了存储引擎的标准行为
// 以后无论是存本地硬盘，还是存阿里云 OSS，都实现这个接口
type StorageEngine interface {
	// Put 将数据流写入存储，返回写入的字节数和可能的错误
	Put(key string, r io.Reader) (int64, error)

	// Get 获取文件流
	Get(key string) (io.ReadCloser, error)

	// Delete 删除文件
	Delete(key string) error

	// Exists 检查文件是否存在
	Exists(key string) (bool, error)
}
