package model

import "gorm.io/gorm"

type UploadTask struct {
	gorm.Model
	UploadID       string `gorm:"uniqueIndex;type:char(32);not null"`
	FileHash       string `gorm:"type:char(64);not null"`
	FileSize       int64  `gorm:"not null"`
	ChunkSize      int    `gorm:"not null"`
	ChunkCount     int    `gorm:"not null"`
	UploadedChunks string `gorm:"type:json"` // 简单起见先存字符串，实际用 hook 转 JSON
	Status         int    `gorm:"default:0"` // 0:上传中, 1:完成
}

func (UploadTask) TableName() string {
	return "upload_tasks"
}
