package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/stywzn/Go-Cloud-Storage/internal/middleware"
	"github.com/stywzn/Go-Cloud-Storage/internal/service"
)

type FileHandler struct {
	svc service.FileService
}

func NewFileHandler(svc service.FileService) *FileHandler {
	return &FileHandler{svc: svc}
}

// UploadHandler 基础单文件上传
func (h *FileHandler) UploadHandler(c *gin.Context) {
	userID := middleware.GetUserID(c)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file"})
		return
	}
	res, err := h.svc.UploadFile(c.Request.Context(), file, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "upload success", "data": res})
}

// InitUpload 初始化分片上传
// 请求参数：file_name, total_size, chunk_size（可选）
// 返回：upload_id, chunk_size
func (h *FileHandler) InitUpload(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fileName := c.PostForm("file_name")
	fileHash := c.PostForm("file_hash")
	totalSize, _ := strconv.ParseInt(c.PostForm("total_size"), 10, 64)

	if fileName == "" || totalSize <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_name or total_size"})
		return
	}

	// 默认分片大小 5MB
	chunkSize := int64(5 * 1024 * 1024)
	chunkSizeStr := c.PostForm("chunk_size")
	if chunkSizeStr != "" {
		if cs, err := strconv.ParseInt(chunkSizeStr, 10, 64); err == nil && cs > 0 {
			chunkSize = cs
		}
	}

	uploadID, respChunkSize, err := h.svc.InitUpload(c.Request.Context(), userID, fileName, totalSize, chunkSize, fileHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"upload_id":  uploadID,
		"chunk_size": respChunkSize,
	})
}

// UploadPart 上传单个分片
// 路由参数：upload_id, part_number
// 表单字段：part（文件）
func (h *FileHandler) UploadPart(c *gin.Context) {
	uploadID := c.Param("upload_id")
	partNumberStr := c.Param("part_number")
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid part_number"})
		return
	}

	file, err := c.FormFile("part")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid part file"})
		return
	}

	src, _ := file.Open()
	defer src.Close()

	err = h.svc.UploadPart(c.Request.Context(), uploadID, partNumber, src, file.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "part uploaded successfully"})
}

// CompleteUpload 完成分片上传，触发合并
// 路由参数：upload_id
func (h *FileHandler) CompleteUpload(c *gin.Context) {
	userID := middleware.GetUserID(c)
	uploadID := c.Param("upload_id")

	res, err := h.svc.CompleteUpload(c.Request.Context(), uploadID, userID)
	if err != nil {
		if err == service.ErrMergeInProgress {
			c.JSON(http.StatusOK, gin.H{
				"msg":       "merge already in progress, please query upload status later",
				"upload_id": uploadID,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

// GetUploadStatus 获取上传状态（已完成的分片列表）
// 路由参数：upload_id
func (h *FileHandler) GetUploadStatus(c *gin.Context) {
	uploadID := c.Param("upload_id")
	status, err := h.svc.GetUploadStatus(c.Request.Context(), uploadID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// GetFileInfo 获取文件元数据
// 路由参数：file_hash
func (h *FileHandler) GetFileInfo(c *gin.Context) {
	fileHash := c.Param("file_hash")
	info, err := h.svc.GetFileInfo(c.Request.Context(), fileHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if info == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// DeleteFile 软删除文件（异步 GC 最终清理物理文件）
// 路由参数：file_hash
func (h *FileHandler) DeleteFile(c *gin.Context) {
	fileHash := c.Param("file_hash")
	if err := h.svc.DeleteFile(c.Request.Context(), fileHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "file soft-deleted, gc will clean physical data asynchronously"})
}
