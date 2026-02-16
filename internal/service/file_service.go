package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"path"

	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
	"github.com/stywzn/Go-Cloud-Storage/internal/storage"
)

// 定义业务逻辑
type FileService interface {
	UploadFile(ctx context.Context, file *multipart.FileHeader) (*model.File, error)
}

type fileService struct {
	repo  repository.FileRepository
	store storage.StorageEngine
}

// NewFileService 函数构造
func NewFileService(repo repository.FileRepository, store storage.StorageEngine) FileService {
	return &fileService{
		repo:  repo,
		store: store,
	}
}

// UploadFile
func (s *fileService) UploadFile(ctx context.Context, fileHeader *multipart.FileHeader) (*model.File, error) {
	//打开文件
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	//计算文件hash
	hash := sha256.New()
	if _, err := io.Copy(hash, src); err != nil {
		return nil, err
	}
	fileHash := hex.EncodeToString(hash.Sum(nil))
	//检查文件是否存在
	existsFile, err := s.repo.GetByHash(ctx, fileHash)
	if err != nil {
		return nil, err
	}
	if existsFile != nil {
		return existsFile, nil
	}
	if _, err := src.Seek(0, 0); err != nil {
		return nil, errors.New("file seek failed")
	}

	ext := path.Ext(fileHeader.Filename)
	storedName := fileHash + ext

	//调用存储引擎存文件(io.Reader)
	if err := s.store.Put(storedName, src, fileHeader.Size); err != nil {
		return nil, err
	}

	//准备数据库模型
	newFile := &model.File{
		OriginalName: fileHeader.Filename,
		StoredName:   storedName,
		Hash:         fileHash,
		Size:         fileHeader.Size,
		Type:         fileHeader.Header.Get("Content-Type"),
	}
	if err := s.repo.Create(ctx, newFile); err != nil {
		return nil, err
	}
	return newFile, nil
}
