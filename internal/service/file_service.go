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
	"math/rand"
	"mime/multipart"
	"sort"
	"strings"
	"time"

	redis "github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stywzn/Go-Cloud-Storage/internal/metrics"
	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
	"github.com/stywzn/Go-Cloud-Storage/internal/storage"
	"github.com/stywzn/Go-Cloud-Storage/pkg/db"
	"github.com/stywzn/Go-Cloud-Storage/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

const (
	fileMetaCachePrefix      = "storage:file:meta:"
	fileMetaCacheTTL         = 10 * time.Minute
	fileMetaCacheJitter      = 5 * time.Minute
	fileMetaCacheEmptyTTL    = 1 * time.Minute
	mergeLockTTL             = 10 * time.Second
	mergeLockRenewInterval   = 5 * time.Second
	mergeLockOpTimeout       = 2 * time.Second
	mergeLockWatchdogTimeout = 2 * time.Second
)

var (
	ErrMergeInProgress = errors.New("merge already in progress")

	releaseLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0`)

	renewLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("EXPIRE", KEYS[1], ARGV[2])
end
return 0`)
)

type FileService interface {
	UploadFile(ctx context.Context, file *multipart.FileHeader, userID uint) (*model.File, error)
	GetFileInfo(ctx context.Context, fileHash string) (*model.File, error)
	DeleteFile(ctx context.Context, fileHash string) error
	// 分片上传接口
	InitUpload(ctx context.Context, userID uint, fileName string, totalSize int64, chunkSize int64, fileHash string) (string, int64, error)
	UploadPart(ctx context.Context, uploadID string, partNumber int, r io.Reader, size int64) error
	CompleteUpload(ctx context.Context, uploadID string, userID uint) (*model.File, error)
	GetUploadStatus(ctx context.Context, uploadID string) (*model.UploadTask, error)
}

type fileService struct {
	repo         repository.FileRepository
	userRepo     repository.UserRepository
	taskRepo     repository.UploadTaskRepository
	store        storage.StorageEngine
	metaSF       singleflight.Group
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

func (s *fileService) fileMetaCacheKey(fileHash string) string {
	return fileMetaCachePrefix + fileHash
}

func (s *fileService) writeFileMetaCache(ctx context.Context, cacheKey string, file *model.File) {
	if db.RDB == nil {
		return
	}

	if file == nil {
		if err := db.RDB.Set(ctx, cacheKey, "null", fileMetaCacheEmptyTTL).Err(); err != nil {
			logger.Log.Warn("write empty file meta cache failed", zap.Error(err), zap.String("cache_key", cacheKey))
		}
		return
	}

	b, err := json.Marshal(file)
	if err != nil {
		logger.Log.Warn("marshal file meta cache payload failed", zap.Error(err), zap.String("cache_key", cacheKey))
		return
	}

	ttl := fileMetaCacheTTL + time.Duration(rand.Int63n(int64(fileMetaCacheJitter)+1))
	if err := db.RDB.Set(ctx, cacheKey, b, ttl).Err(); err != nil {
		logger.Log.Warn("write file meta cache failed", zap.Error(err), zap.String("cache_key", cacheKey))
	}
}

func (s *fileService) invalidateFileMetaCache(ctx context.Context, fileHash string) {
	if db.RDB == nil || fileHash == "" {
		return
	}
	_ = db.RDB.Del(ctx, s.fileMetaCacheKey(fileHash)).Err()
}

func (s *fileService) readFileMetaFromCache(ctx context.Context, cacheKey string) (*model.File, bool, error) {
	if db.RDB == nil {
		return nil, false, nil
	}

	val, err := db.RDB.Get(ctx, cacheKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if val == "null" {
		return nil, true, nil
	}

	var file model.File
	if err := json.Unmarshal([]byte(val), &file); err != nil {
		_ = db.RDB.Del(ctx, cacheKey).Err()
		return nil, false, err
	}
	return &file, true, nil
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
	_, _ = io.Copy(hasher, src)
	hash := hex.EncodeToString(hasher.Sum(nil))

	// 检查文件是否已存在（秒传）
	existingFile, err := s.GetFileInfo(ctx, hash)
	if err != nil {
		return nil, err
	}
	if existingFile != nil {
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

	s.writeFileMetaCache(ctx, s.fileMetaCacheKey(hash), file)

	// 扣减配额
	user.Quota -= fileHeader.Size
	s.userRepo.UpdateUser(ctx, user)

	return file, nil
}

func (s *fileService) GetFileInfo(ctx context.Context, fileHash string) (*model.File, error) {
	fileHash = strings.TrimSpace(strings.ToLower(fileHash))
	if fileHash == "" {
		return nil, errors.New("file hash is required")
	}

	cacheKey := s.fileMetaCacheKey(fileHash)
	if file, hit, err := s.readFileMetaFromCache(ctx, cacheKey); err == nil && hit {
		return file, nil
	} else if err != nil {
		logger.Log.Warn("read file meta cache failed", zap.Error(err), zap.String("cache_key", cacheKey))
	}

	val, err, _ := s.metaSF.Do(cacheKey, func() (interface{}, error) {
		if file, hit, err := s.readFileMetaFromCache(ctx, cacheKey); err == nil && hit {
			return file, nil
		}

		file, err := s.repo.GetByHash(ctx, fileHash)
		if err != nil {
			return nil, err
		}

		s.writeFileMetaCache(ctx, cacheKey, file)
		return file, nil
	})
	if err != nil {
		return nil, err
	}

	if val == nil {
		return nil, nil
	}

	cached, ok := val.(*model.File)
	if !ok {
		return nil, errors.New("invalid cached file meta result")
	}
	return cached, nil
}

func (s *fileService) InitUpload(ctx context.Context, userID uint, fileName string, totalSize int64, chunkSize int64, fileHash string) (string, int64, error) {
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
	fileHash = strings.TrimSpace(strings.ToLower(fileHash))

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
		FileHash:        fileHash,
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

	if task.Status == 1 {
		hash := task.FileHash
		if hash == "" {
			hash = uploadID
		}
		return s.GetFileInfo(ctx, hash)
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

	sort.Ints(completed)

	if db.RDB == nil {
		return nil, errors.New("redis client is not initialized")
	}

	lockTarget := task.FileHash
	if lockTarget == "" {
		lockTarget = uploadID
	}
	lockKey := fmt.Sprintf("storage:file_merge:lock:%s", lockTarget)
	lockValue := uuid.NewString()

	acquired, err := db.RDB.SetNX(ctx, lockKey, lockValue, mergeLockTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("redis error checking lock: %w", err)
	}
	if !acquired {
		return nil, ErrMergeInProgress
	}

	watchdogCtx, cancelWatchdog := context.WithCancel(context.Background())
	go s.runMergeLockWatchdog(watchdogCtx, lockKey, lockValue)
	defer func() {
		cancelWatchdog()

		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), mergeLockOpTimeout)
		defer releaseCancel()
		if _, err := releaseLockScript.Run(releaseCtx, db.RDB, []string{lockKey}, lockValue).Result(); err != nil {
			logger.Log.Warn("release merge lock failed", zap.Error(err), zap.String("lock_key", lockKey))
		}
	}()

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

	fileHash := task.FileHash
	if fileHash == "" {
		fileHash = uploadID
	}

	// 创建文件记录
	file := &model.File{
		OriginalName: task.FileName,
		StoredName:   finalKey,
		Hash:         fileHash,
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

	s.writeFileMetaCache(ctx, s.fileMetaCacheKey(fileHash), file)

	// 更新Prometheus指标
	metrics.RecordUploadComplete("success")
	metrics.UserStorageUsage.WithLabelValues(fmt.Sprintf("%d", userID)).Set(float64(task.FileSize))
	metrics.StorageQuotaRemaining.WithLabelValues(fmt.Sprintf("%d", userID)).Set(float64(user.Quota))

	return file, nil
}

func (s *fileService) runMergeLockWatchdog(ctx context.Context, lockKey, lockValue string) {
	if db.RDB == nil {
		return
	}

	ticker := time.NewTicker(mergeLockRenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			renewCtx, cancel := context.WithTimeout(context.Background(), mergeLockWatchdogTimeout)
			ret, err := renewLockScript.Run(
				renewCtx,
				db.RDB,
				[]string{lockKey},
				lockValue,
				int(mergeLockTTL.Seconds()),
			).Int()
			cancel()
			if err != nil {
				logger.Log.Warn("watchdog renew merge lock failed", zap.Error(err), zap.String("lock_key", lockKey))
				continue
			}
			if ret == 0 {
				logger.Log.Warn("watchdog lost merge lock", zap.String("lock_key", lockKey))
				return
			}
		}
	}
}

func (s *fileService) DeleteFile(ctx context.Context, fileHash string) error {
	fileHash = strings.TrimSpace(strings.ToLower(fileHash))
	if fileHash == "" {
		return errors.New("file hash is required")
	}

	deleted, err := s.repo.SoftDeleteByHash(ctx, fileHash)
	if err != nil {
		return err
	}

	if deleted {
		s.invalidateFileMetaCache(ctx, fileHash)
	}
	return nil
}

func (s *fileService) GetUploadStatus(ctx context.Context, uploadID string) (*model.UploadTask, error) {
	return s.taskRepo.GetTask(ctx, uploadID)
}
