package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
	"github.com/stywzn/Go-Cloud-Storage/internal/storage"
	"github.com/stywzn/Go-Cloud-Storage/pkg/logger"
	"go.uber.org/zap"
)

type FileGCWorker struct {
	repo      repository.FileRepository
	store     storage.StorageEngine
	interval  time.Duration
	grace     time.Duration
	batchSize int
}

func NewFileGCWorker(repo repository.FileRepository, store storage.StorageEngine, interval, grace time.Duration, batchSize int) *FileGCWorker {
	if interval <= 0 {
		interval = time.Hour
	}
	if grace <= 0 {
		grace = 24 * time.Hour
	}
	if batchSize <= 0 {
		batchSize = 200
	}

	return &FileGCWorker{
		repo:      repo,
		store:     store,
		interval:  interval,
		grace:     grace,
		batchSize: batchSize,
	}
}

func (w *FileGCWorker) Start(ctx context.Context) {
	go func() {
		w.runOnce(ctx)

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Log.Info("file gc worker stopped")
				return
			case <-ticker.C:
				w.runOnce(ctx)
			}
		}
	}()
}

func (w *FileGCWorker) runOnce(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	cutoffMs := time.Now().Add(-w.grace).UnixMilli()
	files, err := w.repo.ListSoftDeletedBefore(runCtx, cutoffMs, w.batchSize)
	if err != nil {
		logger.Log.Warn("scan soft-deleted files failed", zap.Error(err))
		return
	}
	if len(files) == 0 {
		return
	}

	for _, file := range files {
		if runCtx.Err() != nil {
			return
		}

		if err := w.deletePhysicalFile(runCtx, &file); err != nil {
			logger.Log.Warn(
				"delete physical file failed",
				zap.Error(err),
				zap.Uint("file_id", uint(file.ID)),
				zap.String("hash", file.Hash),
			)
			continue
		}

		if err := w.repo.HardDeleteByID(runCtx, file.ID); err != nil {
			logger.Log.Warn(
				"hard delete file metadata failed",
				zap.Error(err),
				zap.Uint("file_id", uint(file.ID)),
				zap.String("hash", file.Hash),
			)
			continue
		}

		logger.Log.Info("file gc cleaned", zap.Uint("file_id", uint(file.ID)), zap.String("hash", file.Hash))
	}
}

func (w *FileGCWorker) deletePhysicalFile(ctx context.Context, file *model.File) error {
	if file.FilePath != "" {
		if err := w.store.DeleteObject(ctx, file.FilePath); err != nil {
			return err
		}
	}

	if file.StoredName != "" && file.StoredName != file.FilePath {
		if err := w.store.DeleteObject(ctx, file.StoredName); err != nil {
			return err
		}
	}

	if file.Hash != "" {
		tempPrefix := fmt.Sprintf("temp/%s", file.Hash)
		if err := w.store.DeletePrefix(ctx, tempPrefix); err != nil {
			return err
		}
	}

	if strings.HasSuffix(file.StoredName, ".merged") {
		uploadID := strings.TrimSuffix(file.StoredName, ".merged")
		if uploadID != "" {
			tempPrefix := fmt.Sprintf("temp/%s", uploadID)
			if err := w.store.DeletePrefix(ctx, tempPrefix); err != nil {
				return err
			}
		}
	}

	return nil
}
