package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
	"github.com/stywzn/Go-Cloud-Storage/internal/storage"
)

type FileHandler struct {
	repo  *repository.FileRepository
	store storage.StorageEngine
}

func NewFileHandler(repo *repository.FileRepository, store storage.StorageEngine) *FileHandler {
	return &FileHandler{repo: repo, store: store}
}

// UploadHandler 上传接口
func (h *FileHandler) UploadHandler(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}

	// 打开上传的文件流
	srcFile, _ := fileHeader.Open()
	defer srcFile.Close()

	// 1. 准备 Hash 计算器
	hash := sha256.New()

	// 2. 准备临时文件
	tempPath := filepath.Join("storage", "temp_"+fileHeader.Filename)
	dstFile, _ := os.Create(tempPath)
	defer dstFile.Close()

	// 3. ✨ 核心魔法：MultiWriter ✨
	// 一边写磁盘，一边算 Hash，只读一次 IO
	mw := io.MultiWriter(dstFile, hash)
	size, err := io.Copy(mw, srcFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件写入失败"})
		return
	}

	// 4. 获取最终 Hash
	fileHash := hex.EncodeToString(hash.Sum(nil))

	// 5. 查库：是否秒传？
	existingMeta, err := h.repo.GetFileByHash(fileHash)
	if err == nil {
		// 数据库里有 -> 秒传成功
		os.Remove(tempPath) // 删掉刚才传的临时文件
		c.JSON(http.StatusOK, gin.H{
			"msg":  "🔥 秒传成功",
			"data": existingMeta,
		})
		return
	}

	// 6. 新文件：重命名临时文件 -> 正式文件 (用 Hash 命名)
	finalPath := filepath.Join("storage", fileHash)
	os.Rename(tempPath, finalPath)

	// 7. 入库
	newMeta := &model.FileMeta{
		FileHash: fileHash,
		FileName: fileHeader.Filename,
		FileSize: size,
		FilePath: finalPath,
	}
	h.repo.CreateFileMeta(newMeta)

	c.JSON(http.StatusOK, gin.H{
		"msg":  "✅ 上传成功",
		"data": newMeta,
	})
}

// DownloadHandler 下载接口
func (h *FileHandler) DownloadHandler(c *gin.Context) {
	hash := c.Param("hash")

	meta, err := h.repo.GetFileByHash(hash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	// 恢复原始文件名
	c.Header("Content-Disposition", "attachment; filename="+meta.FileName)
	c.File(meta.FilePath)
}
