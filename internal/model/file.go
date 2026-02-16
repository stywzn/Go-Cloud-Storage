package model

import (
	"gorm.io/gorm"
)

type FileMeta struct {
	gorm.Model
	OriginalName string `json:"original_name" gorm:"type:varchar(255);not null"`
	StoredName   string `json:"stored_name" gorm:"type:varchar(255);not null"`
	Hash         string `json:"hash" gorm:"type:varchar(64);uniqueIndex;not null"`
	Ext          string `json:"ext" gorm:"type:varchar(20)"`
	Size         int64  `json:"size"`
	Type         string `json:"type" gorm:"type:varchar(128)"`
	FilePath     string `gorm:"type:varchar(512);not null" json:"-"` // 物理路径不返回给前端
}

func (FileMeta) TableName() string {
	return "file_metas"
}
