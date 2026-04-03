package service

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"time"

	"github.com/stywzn/Go-Cloud-Storage/internal/metrics"
	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
	"github.com/stywzn/Go-Cloud-Storage/internal/storage"
	"github.com/stywzn/Go-Cloud-Storage/pkg/db"
)

type FileService interface {
	UploadFile(ctx context.Context, file *multipart.FileHeader, userID uint) (*model.File, error)
	// 分片上传接口
	InitUpload(ctx context.Context, userID uint, fileName string, totalSize int64, chunkSize int64) (string, int64, error)
	UploadPart(ctx context.Context, uploadID string, partNumber int, r io.Reader, size int64) error
	CompleteUpload(ctx context.Context, uploadID string, userID uint) (*model.File, error)
	GetUploadStatus(ctx context.Context, uploadID string) (*model.UploadTask, error)
}

type fileService struct {
	repo         repository.FileRepository
	userRepo     repository.UserRepository
	taskRepo     repository.UploadTaskRepository
	store        storage.StorageEngine
	defaultQuota int64 // 默认配额 5GB
}

func NewFileService(
	repo repository.FileRepository,
	userRepo repository.UserRepository,
	taskRepo repository.UploadTaskRepository,
	store storage.StorageEngine,
) FileService {
	return &fileService{
		repo:         repo,
		userRepo:     userRepo,
		taskRepo:     taskRepo,
		store:        store,
		defaultQuota: 5 * 1024 * 1024 * 1024, // 5GB
	}
}

// 生成上传 ID
func generateUploadID() string {
	hasher := md5.New()
	hasher.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (s *fileService) UploadFile(ctx context.Context, fileHeader *multipart.FileHeader, userID uint) (*model.File, error) {
	// 检查用户是否存在（如果不存在，创建默认用户）
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		// 简化：如果用户不存在，返回错误
		return nil, errors.New("user not found")
	}

	// 检查配额
	if user.Quota < fileHeader.Size {
		return nil, errors.New("quota exceeded")
	}

	// 打开文件
	src, err := fileHeader.Open()
	if err != nil {
		return nil, errors.New("cannot open file")
	}
	defer src.Close()

	// 计算 SHA256 哈希
	hasher := sha256.New()
	io.Copy(hasher, src)
	hash := hex.EncodeToString(hasher.Sum(nil))

	// 检查文件是否已存在（秒传）
	existingFile, err := s.repo.GetByHash(ctx, hash)
	if err == nil && existingFile != nil {
		// 文件已存在，直接关联
		return existingFile, nil
	}

	// 重新打开文件用于存储
	src, err = fileHeader.Open()
	if err != nil {
		return nil, errors.New("cannot re-open file")
	}
	defer src.Close()

	// 存储文件
	storedName := hash + ".bin"
	if err := s.store.Put(storedName, src, fileHeader.Size); err != nil {
		return nil, errors.New("failed to save file")
	}

	// 创建文件记录
	file := &model.File{
		OriginalName: fileHeader.Filename,
		StoredName:   storedName,
		Hash:         hash,
		Size:         fileHeader.Size,
		FilePath:     storedName,
	}
	if err := s.repo.Create(ctx, file); err != nil {
		return nil, err
	}

	// 扣减配额
	user.Quota -= fileHeader.Size
	s.userRepo.UpdateUser(ctx, user)

	return file, nil
}

func (s *fileService) InitUpload(ctx context.Context, userID uint, fileName string, totalSize int64, chunkSize int64) (string, int64, error) {
	// 获取用户
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return "", 0, errors.New("user not found")
	}

	// 检查配额
	if user.Quota < totalSize {
		metrics.RecordUploadPartFailure("quota_exceeded")
		return "", 0, errors.New("quota exceeded")
	}

	uploadID := generateUploadID()

	// 计算总chunk数
	totalChunks := (int(totalSize) + int(chunkSize) - 1) / int(chunkSize)

	// 创建upload task记录
	task := &model.UploadTask{
		UserID:          userID,
		UploadID:        uploadID,
		FileName:        fileName,
		FileSize:        totalSize,
		ChunkSize:       chunkSize,
		TotalChunks:     totalChunks,
		CompletedChunks: "[]",
		Status:          0,
		FileHash:        "",
	}

	if err := s.taskRepo.CreateTask(ctx, task); err != nil {
		metrics.RecordUploadPartFailure("create_task_failed")
		return "", 0, err
	}

	// 初始化存储
	if err := s.store.InitUpload(uploadID); err != nil {
		metrics.RecordUploadPartFailure("init_storage_failed")
		s.taskRepo.DeleteTask(ctx, uploadID)
		return "", 0, err
	}

	return uploadID, chunkSize, nil
}

func (s *fileService) UploadPart(ctx context.Context, uploadID string, partNumber int, r io.Reader, size int64) error {
	// 获取upload task
	task, err := s.taskRepo.GetTask(ctx, uploadID)
	if err != nil {
		metrics.RecordUploadPartFailure("task_not_found")
		return errors.New("upload task not found")
	}

	if task.Status != 0 {
		metrics.RecordUploadPartFailure("task_not_uploading")
		return errors.New("upload task not in uploading status")
	}

	// 上传分片
	start := time.Now()
	if err := s.store.UploadPart(uploadID, partNumber, r, size); err != nil {
		metrics.RecordUploadPartFailure("upload_failed")
		return err
	}
	duration := time.Since(start).Seconds()

	// 根据大小范围录制指标
	sizeRange := "small"
	if size > 10*1024*1024 {
		sizeRange = "large"
	}
	metrics.UploadPartDuration.WithLabelValues(sizeRange).Observe(duration)
	metrics.RecordUploadPartSuccess()

	// 更新task的已完成分片列表
	var completed []int
	if err := json.Unmarshal([]byte(task.CompletedChunks), &completed); err != nil {
		completed = []int{}
	}

	// 检查是否已存在
	exists := false
	for _, v := range completed {
		if v == partNumber {
			exists = true
			break
		}
	}
	if !exists {
		completed = append(completed, partNumber)
	}

	completedJSON, _ := json.Marshal(completed)
	task.CompletedChunks = string(completedJSON)
	if err := s.taskRepo.UpdateTask(ctx, task); err != nil {
		return err
	}

	return nil
}

func (s *fileService) CompleteUpload(ctx context.Context, uploadID string, userID uint) (*model.File, error) {
	// 获取upload task
	task, err := s.taskRepo.GetTask(ctx, uploadID)
	if err != nil {
		metrics.RecordUploadComplete("failed")
		return nil, errors.New("upload task not found")
	}

	if task.Status != 0 {
		metrics.RecordUploadComplete("failed")
		return nil, errors.New("upload task not in uploading status")
	}

	// 验证所有分片已上传
	var completed []int
	if err := json.Unmarshal([]byte(task.CompletedChunks), &completed); err != nil {
		completed = []int{}
	}

	if len(completed) != task.TotalChunks {
		metrics.RecordUploadComplete("incomplete_chunks")
		return nil, fmt.Errorf("incomplete chunks: %d/%d", len(completed), task.TotalChunks)
	}

	// ==========================================
	// 【核心优化】：Redis 分布式锁，防御多协程并发合并
	// ==========================================
	lockKey := fmt.Sprintf("lock:merge:%s", uploadID)
	// 尝试获取锁，设置 30 秒超时防死锁
	acquired, err := db.RDB.SetNX(ctx, lockKey, "locked", 30*time.Second).Result()
	if err != nil {
		return nil, fmt.Errorf("redis error checking lock: %v", err)
	}
	if !acquired {
		// 没拿到锁，说明别的网络请求正在合并这个文件，直接阻断脏写
		return nil, errors.New("merge already in progress for this upload")
	}
	// 确保函数退出时（无论成功失败）释放锁
	defer db.RDB.Del(context.Background(), lockKey)
	// ==========================================

	// 调用存储引擎合并分片
	parts := make([]storage.Part, len(completed))
	for i, partNum := range completed {
		parts[i] = storage.Part{PartNumber: partNum}
	}

	// 真正的合并执行处
	finalKey, err := s.store.CompleteUpload(uploadID, parts)
	if err != nil {
		metrics.RecordUploadComplete("failed")
		return nil, err
	}

	// 更新user配额
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	user.Quota -= task.FileSize
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	// 创建文件记录
	file := &model.File{
		OriginalName: task.FileName,
		StoredName:   finalKey,
		Hash:         uploadID,
		Size:         task.FileSize,
		FilePath:     finalKey,
	}
	if err := s.repo.Create(ctx, file); err != nil {
		return nil, err
	}

	// 更新任务状态为已完成
	task.Status = 1
	if err := s.taskRepo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}

	// 更新Prometheus指标
	metrics.RecordUploadComplete("success")
	metrics.UserStorageUsage.WithLabelValues(fmt.Sprintf("%d", userID)).Set(float64(task.FileSize))
	metrics.StorageQuotaRemaining.WithLabelValues(fmt.Sprintf("%d", userID)).Set(float64(user.Quota))

	return file, nil
}

func (s *fileService) GetUploadStatus(ctx context.Context, uploadID string) (*model.UploadTask, error) {
	return s.taskRepo.GetTask(ctx, uploadID)
}
