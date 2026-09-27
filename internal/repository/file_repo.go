package repository

import (
	"context"
	"errors"
	"time"

	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 定义接口
type FileRepository interface {
	Create(ctx context.Context, file *model.File) error
	// 根据 hash 查找文件（仅返回未软删数据）
	GetByHash(ctx context.Context, hash string) (*model.File, error)
	SoftDeleteByHash(ctx context.Context, hash string) (bool, error)
	ListSoftDeletedBefore(ctx context.Context, deletedBeforeMs int64, limit int) ([]model.File, error)
	HardDeleteByID(ctx context.Context, id uint) error
}
type fileRepository struct {
	db *gorm.DB
}

func NewFileRepository(db *gorm.DB) FileRepository {
	return &fileRepository{db: db}
}

// 创建文件
func (r *fileRepository) Create(ctx context.Context, file *model.File) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "hash"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"original_name": file.OriginalName,
				"stored_name":   file.StoredName,
				"ext":           file.Ext,
				"size":          file.Size,
				"type":          file.Type,
				"file_path":     file.FilePath,
				"is_deleted":    0,
				"deleted_at_ms": 0,
				"updated_at":    time.Now(),
			}),
		}).
		Create(file).Error
}

// GetFileByHash
func (r *fileRepository) GetByHash(ctx context.Context, hash string) (*model.File, error) {
	var file model.File
	err := r.db.WithContext(ctx).
		Where("hash = ? AND is_deleted = 0", hash).
		First(&file).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &file, nil
}

func (r *fileRepository) SoftDeleteByHash(ctx context.Context, hash string) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("hash = ? AND is_deleted = 0", hash).
		Updates(map[string]interface{}{
			"is_deleted":    1,
			"deleted_at_ms": time.Now().UnixMilli(),
		})
	return res.RowsAffected > 0, res.Error
}

func (r *fileRepository) ListSoftDeletedBefore(ctx context.Context, deletedBeforeMs int64, limit int) ([]model.File, error) {
	var files []model.File
	query := r.db.WithContext(ctx).
		Where("is_deleted = 1 AND deleted_at_ms > 0 AND deleted_at_ms <= ?", deletedBeforeMs).
		Order("deleted_at_ms ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func (r *fileRepository) HardDeleteByID(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.File{}).Error
}
