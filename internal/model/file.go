package model

import (
	"gorm.io/gorm"
)

type FileMeta struct {
	gorm.Model
	FileHash string `gorm:"uniqueIndex;type:char(64);not null" json:"file_hash"` // 核心：SHA256
	FileName string `gorm:"type:varchar(255);not null" json:"file_name"`
	FileSize int64  `gorm:"not null" json:"file_size"`
	FilePath string `gorm:"type:varchar(512);not null" json:"-"` // 物理路径不返回给前端
}

func (FileMeta) TableName() string {
	return "file_metas"
}
