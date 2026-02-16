package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stywzn/Go-Cloud-Storage/internal/service"
)

type FileHandler struct {
	svc service.FileService
}

func NewFileHandler(svc service.FileService) *FileHandler {
	return &FileHandler{
		svc: svc,
	}
}

// UploadHandler 上传接口
func (h *FileHandler) UploadHandler(c *gin.Context) {
	//解析参数（Gin）
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file"})
		return
	}

	// 调用业务逻辑（Service)
	res, err := h.svc.UploadFile(c.Request.Context(), file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	//返回响应
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"smg":  "upload success",
		"date": res,
	})
}
