package repository

import (
	"context"
	"errors"

	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"gorm.io/gorm"
)

// 定义接口
type FileRepository interface {
	Create(ctx context.Context, file *model.File) error
	//根据hash 查找文件
	GetBtHash(ctx context.Context, hash string) (*model.File, error)
}
type FileRepository struct {
	db *gorm.DB
}

func NewFileRepository(db *gorm.DB) FileRepository {
	return &fileRepository{db: db}
}

func (r *fileRepository) Create(ctx context.Context, file *model.File) error {
	return r.db.WithContext(ctx).Create(file).Error
}

// GetFileByHash
func (r *fileRepository) GetByHash(ctx context.Context, hash string) (*model.File, erro) {
	var file model.File
	err := r.db.WithContext(ctx).Where("hash = ?", hash).First(&file).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &file, nil
}
