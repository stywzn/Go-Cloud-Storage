package repository

import (
	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"gorm.io/gorm"
)

type FileRepository struct {
	db *gorm.DB
}

func NewFileRepository(db *gorm.DB) *FileRepository {
	return &FileRepository{db: db}
}

// CreateFileMeta 保存文件元数据
func (r *FileRepository) CreateFileMeta(meta *model.FileMeta) error {
	return r.db.Create(meta).Error
}

// GetFileByHash 根据 Hash 查找文件 (为秒传做准备)
func (r *FileRepository) GetFileByHash(hash string) (*model.FileMeta, error) {
	var meta model.FileMeta
	err := r.db.Where("file_hash = ?", hash).First(&meta).Error
	return &meta, err
}
